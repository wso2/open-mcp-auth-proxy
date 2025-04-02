package authz

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
)

type asgardeoProvider struct {
	cfg *config.Config
}

// NewAsgardeoProvider initializes a Provider for Asgardeo (demo mode).
func NewAsgardeoProvider(cfg *config.Config) Provider {
	return &asgardeoProvider{cfg: cfg}
}

func (p *asgardeoProvider) WellKnownHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("X-Accel-Buffering", "no")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = forwardedProto
		}
		host := r.Host
		if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}

		baseURL := scheme + "://" + host

		issuer := strings.TrimSuffix(p.cfg.AuthServerBaseURL, "/") + "/token"

		response := map[string]interface{}{
			"issuer":                                issuer,
			"authorization_endpoint":                baseURL + "/authorize",
			"token_endpoint":                        baseURL + "/token",
			"jwks_uri":                              p.cfg.JWKSURL,
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
			"registration_endpoint":                 baseURL + "/register",
			"code_challenge_methods_supported":      []string{"plain", "S256"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Accel-Buffering", "no")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[asgardeoProvider] Error encoding well-known: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func (p *asgardeoProvider) RegisterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("X-Accel-Buffering", "no")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var regReq RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&regReq); err != nil {
			log.Printf("ERROR: reading register request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if len(regReq.RedirectURIs) == 0 {
			http.Error(w, "redirect_uris is required", http.StatusBadRequest)
			return
		}

		// Generate credentials
		regReq.ClientID = "client-" + randomString(8)
		regReq.ClientSecret = randomString(16)

		if err := p.createAsgardeoApplication(regReq); err != nil {
			log.Printf("WARN: Asgardeo application creation failed: %v", err)
			// Optionally http.Error(...) if you want to fail
			// or continue to return partial data.
		}

		resp := RegisterResponse{
			ClientID:      regReq.ClientID,
			ClientSecret:  regReq.ClientSecret,
			ClientName:    regReq.ClientName,
			RedirectURIs:  regReq.RedirectURIs,
			GrantTypes:    regReq.GrantTypes,
			ResponseTypes: regReq.ResponseTypes,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("ERROR: encoding /register response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// ----------------------------------------------------------------
// Asgardeo-specific helpers
// ----------------------------------------------------------------

type RegisterRequest struct {
	ClientID      string   `json:"client_id,omitempty"`
	ClientSecret  string   `json:"client_secret,omitempty"`
	ClientName    string   `json:"client_name"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types,omitempty"`
	ResponseTypes []string `json:"response_types,omitempty"`
}

type RegisterResponse struct {
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	ClientName    string   `json:"client_name"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types"`
	ResponseTypes []string `json:"response_types"`
}

func (p *asgardeoProvider) createAsgardeoApplication(regReq RegisterRequest) error {
	body := buildAsgardeoPayload(regReq)
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal Asgardeo request: %w", err)
	}

	asgardeoAppURL := "https://api.asgardeo.io/t/" + p.cfg.Demo.OrgName + "/api/server/v1/applications"
	req, err := http.NewRequest("POST", asgardeoAppURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Errorf("failed to create Asgardeo API request: %w", err)
	}

	token, err := p.getAsgardeoAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get Asgardeo admin token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("asgardeo API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Asgardeo creation error (%d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("INFO: Created Asgardeo application for clientID=%s", regReq.ClientID)
	return nil
}

func (p *asgardeoProvider) getAsgardeoAdminToken() (string, error) {
	tokenURL := p.cfg.AuthServerBaseURL + "/token"

	formData := "grant_type=client_credentials&scope=internal_application_mgt_create internal_application_mgt_delete " +
		"internal_application_mgt_update internal_application_mgt_view"

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	auth := p.cfg.Demo.ClientID + ":" + p.cfg.Demo.ClientSecret
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   time.Duration(p.cfg.TimeoutSeconds) * time.Second,
		Transport: tr,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token JSON: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func buildAsgardeoPayload(regReq RegisterRequest) map[string]interface{} {
	appName := regReq.ClientName
	if appName == "" {
		appName = "demo-app"
	}
	appName += "-" + randomString(5)

	return map[string]interface{}{
		"name":       appName,
		"templateId": "custom-application-oidc",
		"inboundProtocolConfiguration": map[string]interface{}{
			"oidc": map[string]interface{}{
				"clientId":       regReq.ClientID,
				"clientSecret":   regReq.ClientSecret,
				"grantTypes":     regReq.GrantTypes,
				"callbackURLs":   regReq.RedirectURIs,
				"allowedOrigins": []string{},
				"publicClient":   false,
				"pkce": map[string]bool{
					"mandatory":                      true,
					"supportPlainTransformAlgorithm": true,
				},
				"accessToken": map[string]interface{}{
					"type":                                  "JWT",
					"userAccessTokenExpiryInSeconds":        3600,
					"applicationAccessTokenExpiryInSeconds": 3600,
					"bindingType":                           "cookie",
					"revokeTokensWhenIDPSessionTerminated":  true,
					"validateTokenBinding":                  true,
				},
				"refreshToken": map[string]interface{}{
					"expiryInSeconds":   86400,
					"renewRefreshToken": true,
				},
				"idToken": map[string]interface{}{
					"expiryInSeconds": 3600,
					"audience":        []string{},
					"encryption": map[string]interface{}{
						"enabled":   false,
						"algorithm": "RSA-OAEP",
						"method":    "A128CBC+HS256",
					},
				},
				"logout":                         map[string]interface{}{},
				"validateRequestObjectSignature": false,
			},
		},
		"authenticationSequence": map[string]interface{}{
			"type": "USER_DEFINED",
			"steps": []map[string]interface{}{
				{
					"id": 1,
					"options": []map[string]string{
						{
							"idp":           "Google",
							"authenticator": "GoogleOIDCAuthenticator",
						},
						{
							"idp":           "GitHub",
							"authenticator": "GithubAuthenticator",
						},
						{
							"idp":           "Microsoft",
							"authenticator": "OpenIDConnectAuthenticator",
						},
					},
				},
			},
			"script":          "var onLoginRequest = function(context) {\n    executeStep(1);\n};\n",
			"subjectStepId":   1,
			"attributeStepId": 1,
		},
		"advancedConfigurations": map[string]interface{}{
			"skipLoginConsent":  false,
			"skipLogoutConsent": false,
		},
	}
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
