package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

// NewRouter builds an http.ServeMux that routes
// * /authorize, /token, /register, /.well-known to the provider or proxy
// * MCP paths to the MCP server, etc.
func NewRouter(cfg *config.Config, provider authz.Provider) http.Handler {
	mux := http.NewServeMux()

	modifiers := map[string]RequestModifier{
		"/authorize": &AuthorizationModifier{Config: cfg},
		"/token":     &TokenModifier{Config: cfg},
		"/register":  &RegisterModifier{Config: cfg},
	}

	registeredPaths := make(map[string]bool)

	var defaultPaths []string

	// Handle based on mode configuration
	if cfg.Mode == "demo" || cfg.Mode == "asgardeo" {
		// Demo/Asgardeo mode: Custom handlers for well-known and register
		mux.HandleFunc("/.well-known/oauth-authorization-server", provider.WellKnownHandler())
		registeredPaths["/.well-known/oauth-authorization-server"] = true

		mux.HandleFunc("/register", provider.RegisterHandler())
		registeredPaths["/register"] = true

		// Authorize and token will be proxied with parameter modification
		defaultPaths = []string{"/authorize", "/token"}
	} else {
		// Default provider mode
		if cfg.Default.Path != nil {
			// Check if we have custom response for well-known
			wellKnownConfig, exists := cfg.Default.Path["/.well-known/oauth-authorization-server"]
			if exists && wellKnownConfig.Response != nil {
				// If there's a custom response defined, use our handler
				mux.HandleFunc("/.well-known/oauth-authorization-server", provider.WellKnownHandler())
				registeredPaths["/.well-known/oauth-authorization-server"] = true
			} else {
				// No custom response, add well-known to proxy paths
				defaultPaths = append(defaultPaths, "/.well-known/oauth-authorization-server")
			}

			defaultPaths = append(defaultPaths, "/authorize")
			defaultPaths = append(defaultPaths, "/token")
			defaultPaths = append(defaultPaths, "/register")
		} else {
			defaultPaths = []string{"/authorize", "/token", "/register", "/.well-known/oauth-authorization-server"}
		}
	}

	// Remove duplicates from defaultPaths
	uniquePaths := make(map[string]bool)
	cleanPaths := []string{}
	for _, path := range defaultPaths {
		if !uniquePaths[path] {
			uniquePaths[path] = true
			cleanPaths = append(cleanPaths, path)
		}
	}
	defaultPaths = cleanPaths

	for _, path := range defaultPaths {
		if !registeredPaths[path] {
			mux.HandleFunc(path, buildProxyHandler(cfg, modifiers))
			registeredPaths[path] = true
		}
	}

	// MCP paths
	for _, path := range cfg.MCPPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg, modifiers))
		registeredPaths[path] = true
	}

	// Register paths from PathMapping that haven't been registered yet
	for path := range cfg.PathMapping {
		if !registeredPaths[path] {
			mux.HandleFunc(path, buildProxyHandler(cfg, modifiers))
			registeredPaths[path] = true
		}
	}

	return mux
}

func buildProxyHandler(cfg *config.Config, modifiers map[string]RequestModifier) http.HandlerFunc {
	// Parse the base URLs up front
	authBase, err := url.Parse(cfg.AuthServerBaseURL)
	if err != nil {
		log.Fatalf("Invalid auth server URL: %v", err)
	}
	mcpBase, err := url.Parse(cfg.MCPServerBaseURL)
	if err != nil {
		log.Fatalf("Invalid MCP server URL: %v", err)
	}

	// Detect SSE paths from config
	ssePaths := make(map[string]bool)
	for _, p := range cfg.MCPPaths {
		if p == "/sse" {
			ssePaths[p] = true
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := getAllowedOrigin(origin, cfg)
		// Handle OPTIONS
		if r.Method == http.MethodOptions {
			if allowedOrigin == "" {
				log.Printf("[proxy] Preflight request from disallowed origin: %s", origin)
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}
			addCORSHeaders(w, cfg, allowedOrigin, r.Header.Get("Access-Control-Request-Headers"))
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowedOrigin == "" {
			log.Printf("[proxy] Request from disallowed origin: %s for %s", origin, r.URL.Path)
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		// Add CORS headers to all responses
		addCORSHeaders(w, cfg, allowedOrigin, "")

		// Decide whether the request should go to the auth server or MCP
		var targetURL *url.URL
		isSSE := false

		if isAuthPath(r.URL.Path) {
			targetURL = authBase
		} else if isMCPPath(r.URL.Path, cfg) {
			// Validate JWT for MCP paths if required
			// Placeholder for JWT validation logic
			if err := util.ValidateJWT(r.Header.Get("Authorization")); err != nil {
				log.Printf("[proxy] Unauthorized request to %s: %v", r.URL.Path, err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			targetURL = mcpBase
			if ssePaths[r.URL.Path] {
				isSSE = true
			}
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Apply request modifiers to add parameters
		if modifier, exists := modifiers[r.URL.Path]; exists {
			var err error
			r, err = modifier.ModifyRequest(r)
			if err != nil {
				log.Printf("[proxy] Error modifying request: %v", err)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
		}

		// Build the reverse proxy
		rp := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				// Path rewriting if needed
				mapped := r.URL.Path
				if rewrite, ok := cfg.PathMapping[r.URL.Path]; ok {
					mapped = rewrite
				}
				basePath := strings.TrimRight(targetURL.Path, "/")
				req.URL.Scheme = targetURL.Scheme
				req.URL.Host = targetURL.Host
				req.URL.Path = basePath + mapped
				req.URL.RawQuery = r.URL.RawQuery
				req.Host = targetURL.Host

				cleanHeaders := http.Header{}
				
				// Set proper origin header to match the target
				if isSSE {
					// For SSE, ensure origin matches the target
					req.Header.Set("Origin", targetURL.Scheme+"://"+targetURL.Host)
				}
				
				for k, v := range r.Header {
					// Skip hop-by-hop headers
					if skipHeader(k) {
						continue
					}

					// Set only the first value to avoid duplicates
					cleanHeaders.Set(k, v[0])
				}

				req.Header = cleanHeaders

				log.Printf("[proxy] %s -> %s%s", r.URL.Path, req.URL.Host, req.URL.Path)
			},
			ModifyResponse: func(resp *http.Response) error {
				log.Printf("[proxy] Response from %s%s: %d", resp.Request.URL.Host, resp.Request.URL.Path, resp.StatusCode)
				resp.Header.Del("Access-Control-Allow-Origin") // Avoid upstream conflicts
				return nil
			},
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
				log.Printf("[proxy] Error proxying: %v", err)
				http.Error(rw, "Bad Gateway", http.StatusBadGateway)
			},
			FlushInterval: -1, // immediate flush for SSE
		}

		if isSSE {
			// Add special response handling for SSE connections to rewrite endpoint URLs
			rp.Transport = &sseTransport{
				Transport:  http.DefaultTransport,
				proxyHost:  r.Host,
				targetHost: targetURL.Host,
			}
			
			// Set SSE-specific headers
			w.Header().Set("X-Accel-Buffering", "no")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			
			// Keep SSE connections open
			HandleSSE(w, r, rp)
		} else {
			// Standard requests: enforce a timeout
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.TimeoutSeconds)*time.Second)
			defer cancel()
			rp.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}

func getAllowedOrigin(origin string, cfg *config.Config) string {
	if origin == "" {
		return cfg.CORSConfig.AllowedOrigins[0] // Default to first allowed origin
	}
	for _, allowed := range cfg.CORSConfig.AllowedOrigins {
		log.Printf("[proxy] Checking CORS origin: %s against allowed: %s", origin, allowed)
		if allowed == origin {
			return allowed
		}
	}
	return ""
}

// addCORSHeaders adds configurable CORS headers
func addCORSHeaders(w http.ResponseWriter, cfg *config.Config, allowedOrigin, requestHeaders string) {
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.CORSConfig.AllowedMethods, ", "))
	if requestHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
	} else {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.CORSConfig.AllowedHeaders, ", "))
	}
	if cfg.CORSConfig.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("X-Accel-Buffering", "no")
}

func isAuthPath(path string) bool {
	authPaths := map[string]bool{
		"/authorize": true,
		"/token":     true,
		"/register":  true,
		"/.well-known/oauth-authorization-server": true,
	}
	if strings.HasPrefix(path, "/u/") {
		return true
	}
	return authPaths[path]
}

// isMCPPath checks if the path is an MCP path
func isMCPPath(path string, cfg *config.Config) bool {
	for _, p := range cfg.MCPPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func skipHeader(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "keep-alive", "transfer-encoding", "upgrade", "proxy-authorization", "proxy-connection", "te", "trailer":
		return true
	}
	return false
}