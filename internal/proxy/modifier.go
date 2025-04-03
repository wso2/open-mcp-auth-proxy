package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
)

// RequestModifier modifies requests before they are proxied
type RequestModifier interface {
	ModifyRequest(req *http.Request) (*http.Request, error)
}

// AuthorizationModifier adds parameters to authorization requests
type AuthorizationModifier struct {
	Config *config.Config
}

// TokenModifier adds parameters to token requests
type TokenModifier struct {
	Config *config.Config
}

type RegisterModifier struct {
	Config *config.Config
}

// ModifyRequest adds configured parameters to authorization requests
func (m *AuthorizationModifier) ModifyRequest(req *http.Request) (*http.Request, error) {
	// Check if we have parameters to add
	if m.Config.Default.Path == nil {
		return req, nil
	}

	pathConfig, exists := m.Config.Default.Path["/authorize"]
	if !exists || len(pathConfig.AddQueryParams) == 0 {
		return req, nil
	}
	// Get current query parameters
	query := req.URL.Query()

	// Add parameters from config
	for _, param := range pathConfig.AddQueryParams {
		query.Set(param.Name, param.Value)
	}

	// Update the request URL
	req.URL.RawQuery = query.Encode()

	return req, nil
}

// ModifyRequest adds configured parameters to token requests
func (m *TokenModifier) ModifyRequest(req *http.Request) (*http.Request, error) {
	// Only modify POST requests
	if req.Method != http.MethodPost {
		return req, nil
	}

	// Check if we have parameters to add
	if m.Config.Default.Path == nil {
		return req, nil
	}

	pathConfig, exists := m.Config.Default.Path["/token"]
	if !exists || len(pathConfig.AddBodyParams) == 0 {
		return req, nil
	}

	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse form data
		if err := req.ParseForm(); err != nil {
			return nil, err
		}

		// Clone form data
		formData := req.PostForm

		// Add configured parameters
		for _, param := range pathConfig.AddBodyParams {
			formData.Set(param.Name, param.Value)
		}

		// Create new request body with modified form
		formEncoded := formData.Encode()
		req.Body = io.NopCloser(strings.NewReader(formEncoded))
		req.ContentLength = int64(len(formEncoded))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(formEncoded)))

	} else if strings.Contains(contentType, "application/json") {
		// Read body
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		// Parse JSON
		var jsonData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			return nil, err
		}

		// Add parameters
		for _, param := range pathConfig.AddBodyParams {
			jsonData[param.Name] = param.Value
		}

		// Marshal back to JSON
		modifiedBody, err := json.Marshal(jsonData)
		if err != nil {
			return nil, err
		}

		// Update request
		req.Body = io.NopCloser(bytes.NewReader(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
	}

	return req, nil
}

func (m *RegisterModifier) ModifyRequest(req *http.Request) (*http.Request, error) {
	// Only modify POST requests
	if req.Method != http.MethodPost {
		return req, nil
	}

	// Check if we have parameters to add
	if m.Config.Default.Path == nil {
		return req, nil
	}

	pathConfig, exists := m.Config.Default.Path["/register"]
	if !exists || len(pathConfig.AddBodyParams) == 0 {
		return req, nil
	}

	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse form data
		if err := req.ParseForm(); err != nil {
			return nil, err
		}

		// Clone form data
		formData := req.PostForm

		// Add configured parameters
		for _, param := range pathConfig.AddBodyParams {
			formData.Set(param.Name, param.Value)
		}

		// Create new request body with modified form
		formEncoded := formData.Encode()
		req.Body = io.NopCloser(strings.NewReader(formEncoded))
		req.ContentLength = int64(len(formEncoded))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(formEncoded)))

	} else if strings.Contains(contentType, "application/json") {
		// Read body
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		// Parse JSON
		var jsonData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			return nil, err
		}

		// Add parameters
		for _, param := range pathConfig.AddBodyParams {
			jsonData[param.Name] = param.Value
		}

		// Marshal back to JSON
		modifiedBody, err := json.Marshal(jsonData)
		if err != nil {
			return nil, err
		}

		// Update request
		req.Body = io.NopCloser(bytes.NewReader(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
	}

	return req, nil
}
