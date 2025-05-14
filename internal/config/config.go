package config

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

// Transport mode for MCP server
type TransportMode string

const (
	SSETransport   TransportMode = "sse"
	StdioTransport TransportMode = "stdio"
)

// Common path configuration for all transport modes
type PathsConfig struct {
	SSE            string `yaml:"sse"`
	Messages       string `yaml:"messages"`
	StreamableHTTP string `yaml:"streamable_http"` // Path for streamable HTTP requests
}

// StdioConfig contains stdio-specific configuration
type StdioConfig struct {
	Enabled     bool     `yaml:"enabled"`
	UserCommand string   `yaml:"user_command"`   // The command provided by the user
	WorkDir     string   `yaml:"work_dir"`       // Working directory (optional)
	Args        []string `yaml:"args,omitempty"` // Additional arguments
	Env         []string `yaml:"env,omitempty"`  // Environment variables
}

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
	ListenPort        int    `yaml:"listen_port"`
	BaseURL           string `yaml:"base_url"`
	Port              int    `yaml:"port"`
	JWKSURL           string
	TimeoutSeconds    int               `yaml:"timeout_seconds"`
	PathMapping       map[string]string `yaml:"path_mapping"`
	Mode              string            `yaml:"mode"`
	CORSConfig        CORSConfig        `yaml:"cors"`
	TransportMode     TransportMode     `yaml:"transport_mode"`
	Paths             PathsConfig       `yaml:"paths"`
	Stdio             StdioConfig       `yaml:"stdio"`
	RequiredScopes    map[string]string `yaml:"required_scopes"`

	// Nested config for Asgardeo
	Demo     DemoConfig     `yaml:"demo"`
	Asgardeo AsgardeoConfig `yaml:"asgardeo"`
	Default  DefaultConfig  `yaml:"default"`

	// Protected resource metadata
	Audience               string            `yaml:"audience"`
	ResourceIdentifier     string            `yaml:"resource_identifier"`
	ScopesSupported        map[string]string `yaml:"scopes_supported"`
	AuthorizationServers   []string          `yaml:"authorization_servers"`
	JwksURI                string            `yaml:"jwks_uri,omitempty"`
	BearerMethodsSupported []string          `yaml:"bearer_methods_supported,omitempty"`
}

// Validate checks if the config is valid based on transport mode
func (c *Config) Validate() error {
	// Validate based on transport mode
	if c.TransportMode == StdioTransport {
		if !c.Stdio.Enabled {
			return fmt.Errorf("stdio.enabled must be true in stdio transport mode")
		}
		if c.Stdio.UserCommand == "" {
			return fmt.Errorf("stdio.user_command is required in stdio transport mode")
		}
	}

	// Validate paths
	if c.Paths.SSE == "" {
		c.Paths.SSE = "/sse" // Default value
	}
	if c.Paths.Messages == "" {
		c.Paths.Messages = "/messages" // Default value
	}

	// Validate base URL
	if c.BaseURL == "" {
		if c.Port > 0 {
			c.BaseURL = fmt.Sprintf("http://localhost:%d", c.Port)
		} else {
			c.BaseURL = "http://localhost:8000" // Default value
		}
	}

	return nil
}

// GetMCPPaths returns the list of paths that should be proxied to the MCP server
func (c *Config) GetMCPPaths() []string {
	return []string{c.Paths.SSE, c.Paths.Messages, c.Paths.StreamableHTTP}
}

// BuildExecCommand constructs the full command string for execution in stdio mode
func (c *Config) BuildExecCommand() string {
	if c.Stdio.UserCommand == "" {
		return ""
	}

	if runtime.GOOS == "windows" {
		// For Windows, we need to properly escape the inner command
		escapedCommand := strings.ReplaceAll(c.Stdio.UserCommand, `"`, `\"`)
		return fmt.Sprintf(
			`npx -y supergateway --stdio "%s" --port %d --baseUrl %s --ssePath %s --messagePath %s`,
			escapedCommand, c.Port, c.BaseURL, c.Paths.SSE, c.Paths.Messages,
		)
	}

	return fmt.Sprintf(
		`npx -y supergateway --stdio "%s" --port %d --baseUrl %s --ssePath %s --messagePath %s`,
		c.Stdio.UserCommand, c.Port, c.BaseURL, c.Paths.SSE, c.Paths.Messages,
	)
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

	// Set default values
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 15 // default
	}

	// Set default transport mode if not specified
	if cfg.TransportMode == "" {
		cfg.TransportMode = SSETransport // Default to SSE
	}

	// Set default port if not specified
	if cfg.Port == 0 {
		cfg.Port = 8000 // default
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
