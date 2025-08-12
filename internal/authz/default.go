package authz

import (
	"encoding/json"
	"net/http"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

type defaultProvider struct {
	cfg *config.Config
}

// NewDefaultProvider initializes a Provider for Asgardeo (demo mode).
func NewDefaultProvider(cfg *config.Config) Provider {
	return &defaultProvider{cfg: cfg}
}

func (p *defaultProvider) WellKnownHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if we have a custom response configuration
		if p.cfg.Default.Path != nil {
			pathConfig, exists := p.cfg.Default.Path["/.well-known/oauth-authorization-server"]
			if exists && pathConfig.Response != nil {
				// Use configured response values
				responseConfig := pathConfig.Response

				// Get current host for proxy endpoints
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

				authorizationEndpoint := responseConfig.AuthorizationEndpoint
				if authorizationEndpoint == "" {
					authorizationEndpoint = baseURL + "/authorize"
				}
				tokenEndpoint := responseConfig.TokenEndpoint
				if tokenEndpoint == "" {
					tokenEndpoint = baseURL + "/token"
				}
				registraionEndpoint := responseConfig.RegistrationEndpoint
				if registraionEndpoint == "" {
					registraionEndpoint = baseURL + "/register"
				}

				// Build response from config
				response := map[string]interface{}{
					"issuer":                                responseConfig.Issuer,
					"authorization_endpoint":                authorizationEndpoint,
					"token_endpoint":                        tokenEndpoint,
					"jwks_uri":                              responseConfig.JwksURI,
					"response_types_supported":              responseConfig.ResponseTypesSupported,
					"grant_types_supported":                 responseConfig.GrantTypesSupported,
					"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
					"registration_endpoint":                 registraionEndpoint,
					"code_challenge_methods_supported":      responseConfig.CodeChallengeMethodsSupported,
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("Error encoding well-known response: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}
		}
	}
}

func (p *defaultProvider) RegisterHandler() http.HandlerFunc {
	return nil
}

func (p *defaultProvider) ProtectedResourceMetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		meta := map[string]interface{}{
			"audience":              p.cfg.ProtectedResourceMetadata.Audience,
			"scopes_supported":      p.cfg.ProtectedResourceMetadata.ScopesSupported,
			"authorization_servers": p.cfg.ProtectedResourceMetadata.AuthorizationServers,
		}

		if p.cfg.ProtectedResourceMetadata.JwksURI != "" {
			meta["jwks_uri"] = p.cfg.ProtectedResourceMetadata.JwksURI
		}

		if len(p.cfg.ProtectedResourceMetadata.BearerMethodsSupported) > 0 {
			meta["bearer_methods_supported"] = p.cfg.ProtectedResourceMetadata.BearerMethodsSupported
		}

		if err := json.NewEncoder(w).Encode(meta); err != nil {
			http.Error(w, "failed to encode metadata", http.StatusInternalServerError)
		}
	}
}
