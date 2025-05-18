package util

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
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
		pubKey, err := parseRSAPublicKey(parsed.N, parsed.E)
		if err == nil {
			publicKeys[parsed.Kid] = pubKey
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
func ValidateJWT(
	isLatestSpec bool,
	authHeader, audience string,
) (*authz.TokenClaims, error) {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == "" {
		return nil, errors.New("empty bearer token")
	}

	// Parse & verify the signature
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
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
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("token not valid")
	}

	claimsMap, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unexpected claim type")
	}

	if !isLatestSpec {
		return &authz.TokenClaims{Scopes: nil}, nil
	}

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

	rawScope := claimsMap["scope"]
	scopeList := []string{}
	if s, ok := rawScope.(string); ok {
		scopeList = strings.Fields(s)
	}

	return &authz.TokenClaims{Scopes: scopeList}, nil
}

// Process the required scopes
func GetRequiredScopes(cfg *config.Config, method string) []string {
	if scopes, ok := cfg.ScopesSupported.(map[string]string); ok && len(scopes) > 0 {
		if scope, ok := scopes[method]; ok {
			return []string{scope}
		}
		if parts := strings.SplitN(method, "/", 2); len(parts) > 0 {
			if scope, ok := scopes[parts[0]]; ok {
				return []string{scope}
			}
		}
		return nil
	}

	if scopes, ok := cfg.ScopesSupported.([]string); ok && len(scopes) > 0 {
		return scopes
	}

	return []string{}
}
