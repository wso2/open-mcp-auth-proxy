package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// AsgardeoConfig groups all Asgardeo-specific fields
type DemoConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	OrgName      string `yaml:"org_name"`
}

type AsgardeoConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	OrgName      string `yaml:"org_name"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

type Config struct {
	AuthServerBaseURL string            `yaml:"auth_server_base_url"`
	MCPServerBaseURL  string            `yaml:"mcp_server_base_url"`
	ListenAddress     string            `yaml:"listen_address"`
	JWKSURL           string            `yaml:"jwks_url"`
	TimeoutSeconds    int               `yaml:"timeout_seconds"`
	MCPPaths          []string          `yaml:"mcp_paths"`
	PathMapping       map[string]string `yaml:"path_mapping"`
	Mode              string            `yaml:"mode"`
	CORSConfig        CORSConfig        `yaml:"cors"`

	// Nested config for Asgardeo
	Demo     DemoConfig     `yaml:"demo"`
	Asgardeo AsgardeoConfig `yaml:"asgardeo"`
}

// LoadConfig reads a YAML config file into Config struct.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 15 // default
	}
	return &cfg, nil
}
