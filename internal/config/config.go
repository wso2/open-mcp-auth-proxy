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

type ParamConfig struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ResponseConfig struct {
	Issuer                        string   `yaml:"issuer,omitempty"`
	JwksURI                       string   `yaml:"jwks_uri,omitempty"`
	AuthorizationEndpoint         string   `yaml:"authorization_endpoint,omitempty"`
	TokenEndpoint                 string   `yaml:"token_endpoint,omitempty"`
	RegistrationEndpoint          string   `yaml:"registration_endpoint,omitempty"`
	ResponseTypesSupported        []string `yaml:"response_types_supported,omitempty"`
	GrantTypesSupported           []string `yaml:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported []string `yaml:"code_challenge_methods_supported,omitempty"`
}

type PathConfig struct {
	// For well-known endpoint
	Response *ResponseConfig `yaml:"response,omitempty"`

	// For authorization endpoint
	AddQueryParams []ParamConfig `yaml:"addQueryParams,omitempty"`

	// For token and register endpoints
	AddBodyParams []ParamConfig `yaml:"addBodyParams,omitempty"`
}

type DefaultConfig struct {
	BaseURL string                `yaml:"base_url,omitempty"`
	Path    map[string]PathConfig `yaml:"path,omitempty"`
	JWKSURL string                `yaml:"jwks_url,omitempty"`
}

type Config struct {
	AuthServerBaseURL string
	MCPServerBaseURL  string `yaml:"mcp_server_base_url"`
	ListenPort        int    `yaml:"listen_port"`
	JWKSURL           string
	TimeoutSeconds    int               `yaml:"timeout_seconds"`
	MCPPaths          []string          `yaml:"mcp_paths"`
	PathMapping       map[string]string `yaml:"path_mapping"`
	Mode              string            `yaml:"mode"`
	CORSConfig        CORSConfig        `yaml:"cors"`

	// Nested config for Asgardeo
	Demo     DemoConfig     `yaml:"demo"`
	Asgardeo AsgardeoConfig `yaml:"asgardeo"`
	Default  DefaultConfig  `yaml:"default"`
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
