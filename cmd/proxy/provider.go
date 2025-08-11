package main

import (
	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/constants"
)

func MakeProvider(cfg *config.Config, demoMode, asgardeoMode bool) authz.Provider {
	var mode, orgName string
	switch {
	case demoMode:
		mode = "demo"
		orgName = cfg.Demo.OrgName
	case asgardeoMode:
		mode = "asgardeo"
		orgName = cfg.Asgardeo.OrgName
	default:
		mode = "default"
	}
	cfg.Mode = mode

	switch mode {
	case "demo", "asgardeo":
		if len(cfg.ProtectedResourceMetadata.AuthorizationServers) == 0 && cfg.ProtectedResourceMetadata.JwksURI == "" {
			base := constants.ASGARDEO_BASE_URL + orgName + "/oauth2"
			cfg.AuthServerBaseURL = base
			cfg.JWKSURL = base + "/jwks"
		} else {
			cfg.AuthServerBaseURL = cfg.ProtectedResourceMetadata.AuthorizationServers[0]
			cfg.JWKSURL = cfg.ProtectedResourceMetadata.JwksURI
		}
		return authz.NewAsgardeoProvider(cfg)

	default:
		if cfg.Default.BaseURL != "" && cfg.Default.JWKSURL != "" {
			cfg.AuthServerBaseURL = cfg.Default.BaseURL
			cfg.JWKSURL = cfg.Default.JWKSURL
		} else if len(cfg.ProtectedResourceMetadata.AuthorizationServers) > 0 {
			cfg.AuthServerBaseURL = cfg.ProtectedResourceMetadata.AuthorizationServers[0]
			cfg.JWKSURL = cfg.ProtectedResourceMetadata.JwksURI
		}
		return authz.NewDefaultProvider(cfg)
	}
}
