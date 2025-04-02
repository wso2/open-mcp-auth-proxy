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

	// 1. Custom well-known
	mux.HandleFunc("/.well-known/oauth-authorization-server", provider.WellKnownHandler())

	// 2. Registration
	mux.HandleFunc("/register", provider.RegisterHandler())

	// 3. Default "auth" paths, proxied
	defaultPaths := []string{"/authorize", "/token"}
	for _, path := range defaultPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg))
	}

	// 4. MCP paths
	for _, path := range cfg.MCPPaths {
		mux.HandleFunc(path, buildProxyHandler(cfg))
	}

	// 5. If you want to map additional paths from config.PathMapping
	//    to the same proxy logic:
	for path := range cfg.PathMapping {
		mux.HandleFunc(path, buildProxyHandler(cfg))
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

	// We'll define sets for known auth paths, SSE paths, etc.
	authPaths := map[string]bool{
		"/authorize": true,
		"/token":     true,
		"/.well-known/oauth-authorization-server": true,
	}

	// Detect SSE paths from config
	ssePaths := make(map[string]bool)
	for _, p := range cfg.MCPPaths {
		if p == "/sse" {
			ssePaths[p] = true
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Handle OPTIONS
		if r.Method == http.MethodOptions {
			addCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		addCORSHeaders(w)

		// Decide whether the request should go to the auth server or MCP
		var targetURL *url.URL
		isSSE := false

		if authPaths[r.URL.Path] {
			targetURL = authBase
		} else if isMCPPath(r.URL.Path, cfg) {
			// Validate JWT if you want
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
			// If it's not recognized as an auth path or an MCP path
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

				for header, values := range r.Header {
					// Skip hop-by-hop headers
					if strings.EqualFold(header, "Connection") ||
						strings.EqualFold(header, "Keep-Alive") ||
						strings.EqualFold(header, "Transfer-Encoding") ||
						strings.EqualFold(header, "Upgrade") ||
						strings.EqualFold(header, "Proxy-Authorization") ||
						strings.EqualFold(header, "Proxy-Connection") {
						continue
					}

					for _, value := range values {
						req.Header.Set(header, value)
					}
				}
				log.Printf("[proxy] %s -> %s%s", r.URL.Path, req.URL.Host, req.URL.Path)
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

func addCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func isMCPPath(path string, cfg *config.Config) bool {
	for _, p := range cfg.MCPPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func copyHeaders(src http.Header, dst http.Header) {
	// Exclude hop-by-hop
	hopByHop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
		"Proxy-Authorization": true,
		"Proxy-Connection":    true,
	}
	for k, vv := range src {
		if hopByHop[strings.ToLower(k)] {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
