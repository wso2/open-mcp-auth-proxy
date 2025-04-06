package util

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

type JWKS struct {
	Keys []json.RawMessage `json:"keys"`
}

var publicKeys map[string]*rsa.PublicKey

// FetchJWKS downloads JWKS and stores in a package-level map
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

	publicKeys = make(map[string]*rsa.PublicKey)
	for _, keyData := range jwks.Keys {
		var parsedKey struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Kty string `json:"kty"`
		}
		if err := json.Unmarshal(keyData, &parsedKey); err != nil {
			continue
		}
		if parsedKey.Kty != "RSA" {
			continue
		}
		pubKey, err := parseRSAPublicKey(parsedKey.N, parsedKey.E)
		if err == nil {
			publicKeys[parsedKey.Kid] = pubKey
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

// ValidateJWT checks the Authorization: Bearer token using stored JWKS
func ValidateJWT(authHeader string) error {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return errors.New("missing or invalid Authorization header")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		pubKey, ok := publicKeys[kid]
		if !ok {
			return nil, errors.New("unknown or missing kid in token header")
		}
		return pubKey, nil
	})
	if err != nil {
		return errors.New("invalid token: " + err.Error())
	}
	if !token.Valid {
		return errors.New("invalid token: token not valid")
	}
	return nil
}
