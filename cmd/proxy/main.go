package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/constants"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging" 
	"github.com/wso2/open-mcp-auth-proxy/internal/proxy"
	"github.com/wso2/open-mcp-auth-proxy/internal/subprocess"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

func main() {
	demoMode := flag.Bool("demo", false, "Use Asgardeo-based provider (demo).")
	asgardeoMode := flag.Bool("asgardeo", false, "Use Asgardeo-based provider (asgardeo).")
	debugMode := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	logger.SetDebug(*debugMode)

	// 1. Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Ensure MCPPaths includes the configured paths from the command
	if cfg.Command.Enabled {
		// Add SSE path to MCPPaths if not already present
		ssePath := cfg.Command.SsePath
		if ssePath == "" {
			ssePath = "/sse" // default
		}
		
		messagePath := cfg.Command.MessagePath
		if messagePath == "" {
			messagePath = "/messages" // default
		}
		
		// Make sure paths are in MCPPaths
		ensurePathInList(&cfg.MCPPaths, ssePath)
		ensurePathInList(&cfg.MCPPaths, messagePath)
		
		// Configure baseUrl
		baseUrl := cfg.Command.BaseUrl
		if baseUrl == "" {
			if cfg.Command.Port > 0 {
				baseUrl = fmt.Sprintf("http://localhost:%d", cfg.Command.Port)
			} else {
				baseUrl = "http://localhost:8000" // default
			}
		}
		
		// Add the baseUrl to allowed origins if not already present
		// ensureOriginInList(&cfg.CORSConfig.AllowedOrigins, "http://localhost:8080")
		log.Printf("Using MCP server baseUrl: %s", baseUrl)
	}

	// 3. Start subprocess if configured
	var procManager *subprocess.Manager
	if cfg.Command.Enabled && cfg.Command.UserCommand != "" {
		// Ensure all required dependencies are available
		if err := subprocess.EnsureDependenciesAvailable(cfg.Command.UserCommand); err != nil {
			log.Printf("Warning: %v", err)
			log.Printf("Subprocess may fail to start due to missing dependencies")
		}
    
		procManager = subprocess.NewManager()
		if err := procManager.Start(&cfg.Command); err != nil {
			log.Printf("Warning: Failed to start subprocess: %v", err)
		}
	}

	// 4. Create the chosen provider
	var provider authz.Provider
	if *demoMode {
		cfg.Mode = "demo"
		cfg.AuthServerBaseURL = constants.ASGARDEO_BASE_URL + cfg.Demo.OrgName + "/oauth2"
		cfg.JWKSURL = constants.ASGARDEO_BASE_URL + cfg.Demo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
	} else if *asgardeoMode {
		cfg.Mode = "asgardeo"
		cfg.AuthServerBaseURL = constants.ASGARDEO_BASE_URL + cfg.Asgardeo.OrgName + "/oauth2"
		cfg.JWKSURL = constants.ASGARDEO_BASE_URL + cfg.Asgardeo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
	} else {
		cfg.Mode = "default"
		cfg.JWKSURL = cfg.Default.JWKSURL
		cfg.AuthServerBaseURL = cfg.Default.BaseURL
		provider = authz.NewDefaultProvider(cfg)
	}

	// 5. (Optional) Fetch JWKS if you want local JWT validation
	if err := util.FetchJWKS(cfg.JWKSURL); err != nil {
		log.Fatalf("Failed to fetch JWKS: %v", err)
	}

	// 6. Build the main router
	mux := proxy.NewRouter(cfg, provider)

	listen_address := fmt.Sprintf(":%d", cfg.ListenPort)

	// 7. Start the server
	srv := &http.Server{
		Addr:    listen_address,
		Handler: mux,
	}

	go func() {
		log.Printf("Server listening on %s", listen_address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 8. Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")

	// 9. First terminate subprocess if running
	if procManager != nil && procManager.IsRunning() {
		procManager.Shutdown()
	}

	// 10. Then shutdown the server
	log.Println("Shutting down HTTP server...")
	shutdownCtx, cancel := proxy.NewShutdownContext(5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("Stopped.")
}

// Helper function to ensure a path is in a list
func ensurePathInList(paths *[]string, path string) {
	// Check if path exists in the list
	for _, p := range *paths {
		if p == path {
			return // Path already exists
		}
	}
	// Path doesn't exist, add it
	*paths = append(*paths, path)
	log.Printf("Added path %s to MCPPaths", path)
}

// Helper function to ensure an origin is in a list
func ensureOriginInList(origins *[]string, origin string) {
	// Check if origin exists in the list
	for _, o := range *origins {
		if o == origin {
			return // Origin already exists
		}
	}
	// Origin doesn't exist, add it
	*origins = append(*origins, origin)
	log.Printf("Added %s to allowed CORS origins", origin)
}