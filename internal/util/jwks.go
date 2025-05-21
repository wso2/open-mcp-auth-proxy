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
	accessToken string, 
	audience string,
) error {
	logger.Warn("isLatestSpec: %s", isLatestSpec)
	// Parse & verify the signature
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
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
		logger.Warn("Error detected, returning early")
		return fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		logger.Warn("Token invalid, returning early")
		return errors.New("token not valid")
	}

	claimsMap, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("unexpected claim type")
	}

	if !isLatestSpec {
		return nil
	}

	audRaw, exists := claimsMap["aud"]
	if !exists {
		return errors.New("aud claim missing")
	}
	switch v := audRaw.(type) {
	case string:
		if v != audience {
			return fmt.Errorf("aud %q does not match %q", v, audience)
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
			return fmt.Errorf("audience %v does not include %q", v, audience)
		}
	default:
		return errors.New("aud claim has unexpected type")
	}

	return nil
}

// Parses the JWT token and returns the claims
func ParseJWT(tokenStr string) (jwt.MapClaims, error) {
    if tokenStr == "" {
        return nil, fmt.Errorf("empty JWT")
    }

    var claims jwt.MapClaims
    _, _, err := jwt.NewParser().ParseUnverified(tokenStr, &claims)
    if err != nil {
        return nil, fmt.Errorf("failed to parse JWT: %w", err)
    }
    return claims, nil
}

// Process the required scopes
func GetRequiredScopes(cfg *config.Config, method string) []string {
    switch raw := cfg.ScopesSupported.(type) {
    case map[string]string:
        if scope, ok := raw[method]; ok {
            return []string{scope}
        }
        parts := strings.SplitN(method, "/", 2)
        if len(parts) > 0 {
            if scope, ok := raw[parts[0]]; ok {
                return []string{scope}
            }
        }
        return nil
    case []interface{}:
        out := make([]string, 0, len(raw))
        for _, v := range raw {
            if s, ok := v.(string); ok && s != "" {
                out = append(out, s)
            }
        }
        return out

    case []string:
        return raw
    }

    return nil
}

// Extracts the Bearer token from the Authorization header
func ExtractAccessToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("empty authorization header")
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("invalid authorization header format: %s", authHeader)
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if tokenStr == "" {
		return "", errors.New("empty bearer token")
	}

	return tokenStr, nil
}
