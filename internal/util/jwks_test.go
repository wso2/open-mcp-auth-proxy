package util

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestValidateJWT(t *testing.T) {
	// Initialize the test JWKS data
	initTestJWKS(t)

	// Test cases
	tests := []struct {
		name        string
		authHeader  string
		expectError bool
	}{
		{
			name:        "Valid JWT token",
			authHeader:  "Bearer " + createValidJWT(t),
			expectError: false,
		},
		{
			name:        "No auth header",
			authHeader:  "",
			expectError: true,
		},
		{
			name:        "Invalid auth header format",
			authHeader:  "InvalidFormat",
			expectError: true,
		},
		{
			name:        "Invalid JWT token",
			authHeader:  "Bearer invalid.jwt.token",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateJWT(tc.authHeader)
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestFetchJWKS(t *testing.T) {
	// Create a mock JWKS server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a test RSA key
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate RSA key: %v", err)
		}

		// Create JWKS response
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": "test-key-id",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}), // Default exponent 65537
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Test fetching JWKS
	err := FetchJWKS(server.URL)
	if err != nil {
		t.Fatalf("FetchJWKS failed: %v", err)
	}

	// Check that keys were stored
	if len(publicKeys) == 0 {
		t.Errorf("Expected publicKeys to be populated")
	}
}

// Helper function to initialize test JWKS data
func initTestJWKS(t *testing.T) {
	// Create a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Initialize the publicKeys map
	publicKeys = map[string]*rsa.PublicKey{
		"test-key-id": &privateKey.PublicKey,
	}
}

// Helper function to create a valid JWT token for testing
func createValidJWT(t *testing.T) string {
	// Create a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Ensure the test key is in the publicKeys map
	if publicKeys == nil {
		publicKeys = map[string]*rsa.PublicKey{}
	}
	publicKeys["test-key-id"] = &privateKey.PublicKey

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  "1234567890",
		"name": "Test User",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "test-key-id"

	// Sign the token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	return tokenString
}
