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

	registeredPaths := make(map[string]bool)

	var defaultPaths []string
	if cfg.Mode == "demo" || cfg.Mode == "asgardeo" {
		// 1. Custom well-known
		mux.HandleFunc("/.well-known/oauth-authorization-server", provider.WellKnownHandler())
		registeredPaths["/.well-known/oauth-authorization-server"] = true

		// 2. Registration
		mux.HandleFunc("/register", provider.RegisterHandler())
		registeredPaths["/register"] = true

		defaultPaths = []string{"/authorize", "/token"}
	} else {
		defaultPaths = []string{"/authorize", "/token", "/register", "/.well-known/oauth-authorization-server"}
	}

	for _, path := range defaultPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg))
		registeredPaths[path] = true
	}

	// 4. MCP paths
	for _, path := range cfg.MCPPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg))
		registeredPaths[path] = true
	}

	// 5. Register paths from PathMapping that haven't been registered yet
	for path := range cfg.PathMapping {
		if !registeredPaths[path] {
			mux.HandleFunc(path, buildProxyHandler(cfg))
			registeredPaths[path] = true
		}
	}

	return mux
}

func buildProxyHandler(cfg *config.Config) http.HandlerFunc {
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

				for k, v := range r.Header {
					// Skip hop-by-hop headers
					if skipHeader(k) {
						continue
					}

					// Set only the first value to avoid duplicates
					cleanHeaders.Set(k, v[0])
				}

				// Override or remove sensitive headers if needed
				if strings.Contains(req.URL.Path, "/token") {
					cleanHeaders.Set("Accept", "application/json")
					cleanHeaders.Set("Content-Type", "application/x-www-form-urlencoded")
					cleanHeaders.Set("User-Agent", "GoProxy/1.0")
					cleanHeaders.Del("Origin")
					cleanHeaders.Del("Referer")
				}

				req.Header = cleanHeaders

				// DEBUG: log headers sent to Asgardeo
				log.Println("[proxy] Outgoing request headers:")
				for k, v := range req.Header {
					log.Printf("  %s: %s", k, strings.Join(v, ", "))
				}

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
}

func isAuthPath(path string) bool {
	authPaths := map[string]bool{
		"/authorize": true,
		"/token":     true,
		"/.well-known/oauth-authorization-server": true,
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
