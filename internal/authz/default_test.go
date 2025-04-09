package authz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
)

func TestNewDefaultProvider(t *testing.T) {
	cfg := &config.Config{}
	provider := NewDefaultProvider(cfg)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// Ensure it implements the Provider interface
	var _ Provider = provider
}

func TestDefaultProviderWellKnownHandler(t *testing.T) {
	// Create a config with a custom well-known response
	cfg := &config.Config{
		Default: config.DefaultConfig{
			Path: map[string]config.PathConfig{
				"/.well-known/oauth-authorization-server": {
					Response: &config.ResponseConfig{
						Issuer:                        "https://test-issuer.com",
						JwksURI:                       "https://test-issuer.com/jwks",
						ResponseTypesSupported:        []string{"code"},
						GrantTypesSupported:           []string{"authorization_code"},
						CodeChallengeMethodsSupported: []string{"S256"},
					},
				},
			},
		},
	}

	provider := NewDefaultProvider(cfg)
	handler := provider.WellKnownHandler()

	// Create a test request
	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	req.Host = "test-host.com"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", w.Code)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got %s", contentType)
	}

	// Decode and check the response body
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// Check expected values
	if response["issuer"] != "https://test-issuer.com" {
		t.Errorf("Expected issuer=https://test-issuer.com, got %v", response["issuer"])
	}
	if response["jwks_uri"] != "https://test-issuer.com/jwks" {
		t.Errorf("Expected jwks_uri=https://test-issuer.com/jwks, got %v", response["jwks_uri"])
	}
	if response["authorization_endpoint"] != "https://test-host.com/authorize" {
		t.Errorf("Expected authorization_endpoint=https://test-host.com/authorize, got %v", response["authorization_endpoint"])
	}
}

func TestDefaultProviderHandleOPTIONS(t *testing.T) {
	provider := NewDefaultProvider(&config.Config{})
	handler := provider.WellKnownHandler()

	// Create OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handler(w, req)

	// Check response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status NoContent for OPTIONS request, got %v", w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
		t.Errorf("Expected Access-Control-Allow-Methods: GET, OPTIONS, got %s", w.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestDefaultProviderInvalidMethod(t *testing.T) {
	provider := NewDefaultProvider(&config.Config{})
	handler := provider.WellKnownHandler()

	// Create POST request (which should be rejected)
	req := httptest.NewRequest("POST", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handler(w, req)

	// Check response
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status MethodNotAllowed for POST request, got %v", w.Code)
	}
}
