package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/proxy"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

func main() {
	demoMode := flag.Bool("demo", false, "Use Asgardeo-based provider (demo).")
	asgardeoMode := flag.Bool("asgardeo", false, "Use Asgardeo-based provider (demo).")
	flag.Parse()

	// 1. Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Create the chosen provider
	var provider authz.Provider
	if *demoMode {
		cfg.AuthServerBaseURL = "https://api.asgardeo.io/t/" + cfg.Demo.OrgName + "/oauth2"
		cfg.JWKSURL = "https://api.asgardeo.io/t/" + cfg.Demo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
		fmt.Println("Using Asgardeo provider (demo).")
	} else if *asgardeoMode {
		cfg.AuthServerBaseURL = "https://api.asgardeo.io/t/" + cfg.Asgardeo.OrgName + "/oauth2"
		cfg.JWKSURL = "https://api.asgardeo.io/t/" + cfg.Asgardeo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
	}

	// 3. (Optional) Fetch JWKS if you want local JWT validation
	if err := util.FetchJWKS(cfg.JWKSURL); err != nil {
		log.Fatalf("Failed to fetch JWKS: %v", err)
	}

	// 4. Build the main router
	mux := proxy.NewRouter(cfg, provider)

	// 5. Start the server
	srv := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: mux,
	}

	go func() {
		log.Printf("Server listening on %s", cfg.ListenAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 6. Graceful shutdown on Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Shutting down...")

	shutdownCtx, cancel := proxy.NewShutdownContext(5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Stopped.")
}
