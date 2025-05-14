package util

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/constants"
	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

type TokenClaims struct {
	Scopes []string
}

type JWKS struct {
	Keys []json.RawMessage `json:"keys"`
}

var publicKeys map[string]*rsa.PublicKey

// FetchJWKS downloads JWKS and stores in a package‐level map
func FetchJWKS(jwksURL string) error {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	publicKeys = make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, keyData := range jwks.Keys {
		var parsed struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Kty string `json:"kty"`
		}
		if err := json.Unmarshal(keyData, &parsed); err != nil {
			continue
		}
		if parsed.Kty != "RSA" {
			continue
		}
		pk, err := parseRSAPublicKey(parsed.N, parsed.E)
		if err == nil {
			publicKeys[parsed.Kid] = pk
		}
	}
	logger.Info("Loaded %d public keys.", len(publicKeys))
	return nil
}

func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := jwt.DecodeSegment(nStr)
	if err != nil {
		return nil, err
	}
	eBytes, err := jwt.DecodeSegment(eStr)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// ValidateJWT checks the Bearer token according to the Mcp-Protocol-Version.
//   - versionHeader: the raw value of the "Mcp-Protocol-Version" header
//   - authHeader:    the full "Authorization" header
//   - audience:      the resource identifier to check "aud" against
//   - requiredScope: the single scope required (empty ⇒ skip scope check)
func ValidateJWT(
	versionHeader, authHeader, audience, requiredScope string,
) (*authz.TokenClaims, error) {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == "" {
		return nil, errors.New("empty bearer token")
	}

	// 2) parse & verify signature
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		pk, ok := publicKeys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		return pk, nil
	})

	logger.Info("token: %v", token)
	logger.Info("err: %v", err)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("token not valid")
	}

	// always extract claims
	claimsMap, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unexpected claim type")
	}

	// parse version date
	verDate, err := time.Parse("2006-01-02", versionHeader)
	if err != nil {
		// if unparsable or missing, assume _old_ spec
		verDate = time.Time{} // zero time ⇒ before cutover
	}

	// if older than cutover, skip audience+scope
	if verDate.Before(constants.SpecCutoverDate) {
		return &authz.TokenClaims{Scopes: nil}, nil
	}

	// --- new spec flow: enforce audience ---
	audRaw, exists := claimsMap["aud"]
	if !exists {
		return nil, errors.New("aud claim missing")
	}
	switch v := audRaw.(type) {
	case string:
		if v != audience {
			return nil, fmt.Errorf("aud %q does not match %q", v, audience)
		}
	case []interface{}:
		var found bool
		for _, a := range v {
			if s, ok := a.(string); ok && s == audience {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("audience %v does not include %q", v, audience)
		}
	default:
		return nil, errors.New("aud claim has unexpected type")
	}

	// if no scope required, we're done
	if requiredScope == "" {
		return &authz.TokenClaims{Scopes: nil}, nil
	}

	// enforce scope
	rawScope, exists := claimsMap["scope"]
	if !exists {
		return nil, errors.New("scope claim missing")
	}
	scopeStr, ok := rawScope.(string)
	if !ok {
		return nil, errors.New("scope claim not a string")
	}
	scopes := strings.Fields(scopeStr)
	for _, s := range scopes {
		if s == requiredScope {
			return &authz.TokenClaims{Scopes: scopes}, nil
		}
	}
	return nil, fmt.Errorf("insufficient scope: %q not in %v", requiredScope, scopes)
}

// Performs basic JWT validation
func ValidateJWTLegacy(authHeader string) error {
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found")
		}
		key, ok := publicKeys[kid]
		if !ok {
			return nil, fmt.Errorf("key not found for kid: %s", kid)
		}
		return key, nil
	})
	return err
}
