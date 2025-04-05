package main

import (
	"flag"
	"fmt"
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
	stdioMode := flag.Bool("stdio", false, "Use stdio transport mode instead of SSE")
	flag.Parse()

	logger.SetDebug(*debugMode)

	// 1. Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Error("Error loading config: %v", err)
		os.Exit(1)
	}

	// Override transport mode if stdio flag is set
	if *stdioMode {
		cfg.TransportMode = config.StdioTransport
		// Validate command config for stdio mode
		if err := cfg.Command.Validate(cfg.TransportMode); err != nil {
			logger.Error("Configuration error: %v", err)
			os.Exit(1)
		}
	}

	// 2. Ensure MCPPaths are properly configured
	if cfg.TransportMode == config.StdioTransport && cfg.Command.Enabled {
		// Use command.base_url for MCPServerBaseURL in stdio mode
		cfg.MCPServerBaseURL = cfg.Command.GetBaseURL()
		
		// Use command paths for MCPPaths in stdio mode
		cfg.MCPPaths = cfg.Command.GetPaths()
		
		logger.Info("Using MCP server baseUrl: %s", cfg.MCPServerBaseURL)
		logger.Info("Using MCP paths: %v", cfg.MCPPaths)
	}

	// 3. Start subprocess if configured and in stdio mode
	var procManager *subprocess.Manager
	if cfg.TransportMode == config.StdioTransport && cfg.Command.Enabled && cfg.Command.UserCommand != "" {
		// Ensure all required dependencies are available
		if err := subprocess.EnsureDependenciesAvailable(cfg.Command.UserCommand); err != nil {
			logger.Warn("%v", err)
			logger.Warn("Subprocess may fail to start due to missing dependencies")
		}
    
		procManager = subprocess.NewManager()
		if err := procManager.Start(&cfg.Command); err != nil {
			logger.Warn("Failed to start subprocess: %v", err)
		}
	} else if cfg.TransportMode == config.SSETransport {
		logger.Info("Using SSE transport mode, not starting subprocess")
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
		logger.Error("Failed to fetch JWKS: %v", err)
		os.Exit(1)
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
		logger.Info("Server listening on %s", listen_address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	// 8. Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down...")

	// 9. First terminate subprocess if running
	if procManager != nil && procManager.IsRunning() {
		procManager.Shutdown()
	}

	// 10. Then shutdown the server
	logger.Info("Shutting down HTTP server...")
	shutdownCtx, cancel := proxy.NewShutdownContext(5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	}
	logger.Info("Stopped.")
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
	logger.Info("Added path %s to MCPPaths", path)
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
	logger.Info("Added %s to allowed CORS origins", origin)
}