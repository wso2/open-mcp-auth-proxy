package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

// NewRouter builds an http.ServeMux that routes
// * /authorize, /token, /register, /.well-known to the provider or proxy
// * MCP paths to the MCP server, etc.
func NewRouter(cfg *config.Config, provider authz.Provider, accessController authz.AccessControl) http.Handler {
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

	mux.HandleFunc("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := getAllowedOrigin(origin, cfg)
		if r.Method == http.MethodOptions {
			addCORSHeaders(w, cfg, allowed, r.Header.Get("Access-Control-Request-Headers"))
			w.WriteHeader(http.StatusNoContent)
			return
		}

		addCORSHeaders(w, cfg, allowed, "")
		provider.ProtectedResourceMetadataHandler()(w, r)
	})
	registeredPaths["/.well-known/oauth-protected-resource"] = true

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
			mux.HandleFunc(path, buildProxyHandler(cfg, modifiers, accessController))
			registeredPaths[path] = true
		}
	}

	// MCP paths
	mcpPaths := cfg.GetMCPPaths()
	for _, path := range mcpPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg, modifiers, accessController))
		registeredPaths[path] = true
	}

	// Register paths from PathMapping that haven't been registered yet
	for path := range cfg.PathMapping {
		if !registeredPaths[path] {
			mux.HandleFunc(path, buildProxyHandler(cfg, modifiers, accessController))
			registeredPaths[path] = true
		}
	}

	return mux
}

func buildProxyHandler(cfg *config.Config, modifiers map[string]RequestModifier, accessController authz.AccessControl) http.HandlerFunc {
	// Parse the base URLs up front
	authBase, err := url.Parse(cfg.AuthServerBaseURL)
	if err != nil {
		logger.Error("Invalid auth server URL: %v", err)
		panic(err) // Fatal error that prevents startup
	}

	mcpBase, err := url.Parse(cfg.BaseURL)
	if err != nil {
		logger.Error("Invalid MCP server URL: %v", err)
		panic(err) // Fatal error that prevents startup
	}

	// Detect SSE paths from config
	ssePaths := make(map[string]bool)
	ssePaths[cfg.Paths.SSE] = true

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := getAllowedOrigin(origin, cfg)
		// Handle OPTIONS
		if r.Method == http.MethodOptions {
			if allowedOrigin == "" {
				logger.Warn("Preflight request from disallowed origin: %s", origin)
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}
			addCORSHeaders(w, cfg, allowedOrigin, r.Header.Get("Access-Control-Request-Headers"))
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowedOrigin == "" {
			logger.Warn("Request from disallowed origin: %s for %s", origin, r.URL.Path)
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		// Add CORS headers to all responses
		addCORSHeaders(w, cfg, allowedOrigin, "")

		// Check if the request is for the latest spec
		specVersion := util.GetVersionWithDefault(r.Header.Get("MCP-Protocol-Version"))
		ver, err := util.ParseVersionDate(specVersion)
		isLatestSpec := util.IsLatestSpec(ver, err)

		// Decide whether the request should go to the auth server or MCP
		var targetURL *url.URL
		isSSE := false

		if isAuthPath(r.URL.Path) {
			targetURL = authBase
		} else if isMCPPath(r.URL.Path, cfg) {
			if ssePaths[r.URL.Path] {
				if err := authorizeSSE(w, r, isLatestSpec, cfg.ResourceIdentifier); err != nil {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				isSSE = true
			} else {
				if err := authorizeMCP(w, r, isLatestSpec, cfg, accessController); err != nil {
					http.Error(w, err.Error(), http.StatusForbidden)
					return
				}
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
				logger.Error("Error modifying request: %v", err)
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

				logger.Debug("%s -> %s%s", r.URL.Path, req.URL.Host, req.URL.Path)
			},
			ModifyResponse: func(resp *http.Response) error {
				logger.Debug("Response from %s%s: %d", resp.Request.URL.Host, resp.Request.URL.Path, resp.StatusCode)
				if resp.StatusCode == http.StatusUnauthorized {
					resp.Header.Set(
						"WWW-Authenticate",
						fmt.Sprintf(
							`Bearer resource_metadata="%s"`,
							cfg.ResourceIdentifier+"/.well-known/oauth-protected-resource",
						))
					resp.Header.Set("Access-Control-Expose-Headers", "WWW-Authenticate")
				}

				resp.Header.Del("Access-Control-Allow-Origin")
				return nil
			},
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
				logger.Error("Error proxying: %v", err)
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
			w.Header().Set("Content-Type", "text/event-stream")
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

// Check if the request is for SSE handshake and authorize it
func authorizeSSE(w http.ResponseWriter, r *http.Request, isLatestSpec bool, resourceID string) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		if isLatestSpec {
			realm := resourceID + "/.well-known/oauth-protected-resource"
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s"`, realm))
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate")
		}
		return fmt.Errorf("missing or invalid Authorization header")
	}

	return nil
}

// Handles both v1 (just signature) and v2 (aud + scope) flows
func authorizeMCP(w http.ResponseWriter, r *http.Request, isLatestSpec bool, cfg *config.Config, accessController authz.AccessControl) error {
	authzHeader := r.Header.Get("Authorization")
	accessToken, _ := util.ExtractAccessToken(authzHeader)
	if !strings.HasPrefix(authzHeader, "Bearer ") {
		if isLatestSpec {
			realm := cfg.ResourceIdentifier + "/.well-known/oauth-protected-resource"
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer resource_metadata=%q`, realm,
			))
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate")
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return fmt.Errorf("missing or invalid Authorization header")
	}

	err := util.ValidateJWT(isLatestSpec, accessToken, cfg.Audience)
	if err != nil {
		if isLatestSpec {
			realm := cfg.ResourceIdentifier + "/.well-known/oauth-protected-resource"
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(err.Error(),
				`Bearer realm=%q`,
				realm,
			))
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate")
			http.Error(w, "Forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return err
	}

	if isLatestSpec {
		_, err := util.ParseRPCRequest(r)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return err
		}

		claimsMap, err := util.ParseJWT(accessToken)
		if err != nil {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return fmt.Errorf("invalid token claims")
		}
		
		pr := accessController.ValidateAccess(r, &claimsMap, cfg)
		if pr.Decision == authz.DecisionDeny {
			http.Error(w, "Forbidden: "+pr.Message, http.StatusForbidden)
			return fmt.Errorf("forbidden — %s", pr.Message)
		}
	}

	return nil
}

func getAllowedOrigin(origin string, cfg *config.Config) string {
	if origin == "" {
		return cfg.CORSConfig.AllowedOrigins[0] // Default to first allowed origin
	}
	for _, allowed := range cfg.CORSConfig.AllowedOrigins {
		logger.Debug("Checking CORS origin: %s against allowed: %s", origin, allowed)
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
	w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, MCP-Protocol-Version")
	if requestHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
	} else {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.CORSConfig.AllowedHeaders, ", "))
	}
	if cfg.CORSConfig.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("MCP-Protocol-Version", ", ")
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
		"/.well-known/oauth-protected-resource":   true,
	}
	if strings.HasPrefix(path, "/u/") {
		return true
	}
	return authPaths[path]
}

// isMCPPath checks if the path is an MCP path
func isMCPPath(path string, cfg *config.Config) bool {
	mcpPaths := cfg.GetMCPPaths()
	for _, p := range mcpPaths {
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
