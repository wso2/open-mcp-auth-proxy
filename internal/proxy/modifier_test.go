package proxy

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
)

func TestAuthorizationModifier(t *testing.T) {
	cfg := &config.Config{
		Default: config.DefaultConfig{
			Path: map[string]config.PathConfig{
				"/authorize": {
					AddQueryParams: []config.ParamConfig{
						{Name: "client_id", Value: "test-client-id"},
						{Name: "scope", Value: "openid"},
					},
				},
			},
		},
	}

	modifier := &AuthorizationModifier{Config: cfg}

	// Create a test request
	req, err := http.NewRequest("GET", "/authorize?response_type=code", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Modify the request
	modifiedReq, err := modifier.ModifyRequest(req)
	if err != nil {
		t.Fatalf("ModifyRequest failed: %v", err)
	}

	// Check that the query parameters were added
	query := modifiedReq.URL.Query()
	if query.Get("client_id") != "test-client-id" {
		t.Errorf("Expected client_id=test-client-id, got %s", query.Get("client_id"))
	}
	if query.Get("scope") != "openid" {
		t.Errorf("Expected scope=openid, got %s", query.Get("scope"))
	}
	if query.Get("response_type") != "code" {
		t.Errorf("Expected response_type=code, got %s", query.Get("response_type"))
	}
}

func TestTokenModifier(t *testing.T) {
	cfg := &config.Config{
		Default: config.DefaultConfig{
			Path: map[string]config.PathConfig{
				"/token": {
					AddBodyParams: []config.ParamConfig{
						{Name: "audience", Value: "test-audience"},
					},
				},
			},
		},
	}

	modifier := &TokenModifier{Config: cfg}

	// Create a test request with form data
	form := url.Values{}

	req, err := http.NewRequest("POST", "/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Modify the request
	modifiedReq, err := modifier.ModifyRequest(req)
	if err != nil {
		t.Fatalf("ModifyRequest failed: %v", err)
	}

	body := make([]byte, 1024)
	n, err := modifiedReq.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Failed to read body: %v", err)
	}
	bodyStr := string(body[:n])

	// Parse the form data from the modified request
	if err := modifiedReq.ParseForm(); err != nil {
		t.Fatalf("Failed to parse form data: %v", err)
	}

	// Check that the body parameters were added
	if !strings.Contains(bodyStr, "audience") {
		t.Errorf("Expected body to contain audience, got %s", bodyStr)
	}
}

func TestRegisterModifier(t *testing.T) {
	cfg := &config.Config{
		Default: config.DefaultConfig{
			Path: map[string]config.PathConfig{
				"/register": {
					AddBodyParams: []config.ParamConfig{
						{Name: "client_name", Value: "test-client"},
					},
				},
			},
		},
	}

	modifier := &RegisterModifier{Config: cfg}

	// Create a test request with JSON data
	jsonBody := `{"redirect_uris":["https://example.com/callback"]}`
	req, err := http.NewRequest("POST", "/register", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Modify the request
	modifiedReq, err := modifier.ModifyRequest(req)
	if err != nil {
		t.Fatalf("ModifyRequest failed: %v", err)
	}

	// Read the body and check that it still contains the original data
	// This test would need to be enhanced with a proper JSON parsing to verify
	// the added parameters
	body := make([]byte, 1024)
	n, err := modifiedReq.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Failed to read body: %v", err)
	}
	bodyStr := string(body[:n])

	// Simple check to see if the modified body contains the expected fields
	if !strings.Contains(bodyStr, "client_name") {
		t.Errorf("Expected body to contain client_name, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "redirect_uris") {
		t.Errorf("Expected body to contain redirect_uris, got %s", bodyStr)
	}
}
