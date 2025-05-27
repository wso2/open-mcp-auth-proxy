package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	// Basic valid config
	validConfig := `
listen_port: 8080
base_url: "http://localhost:8000"
transport_mode: "sse"
paths:
  sse: "/sse"
  messages: "/messages"
cors:
  allowed_origins:
    - "http://localhost:5173"
  allowed_methods:
    - "GET"
    - "POST"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  allow_credentials: true
`
	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading the valid config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	// Verify expected values from the config
	if cfg.ListenPort != 8080 {
		t.Errorf("Expected ListenPort=8080, got %d", cfg.ListenPort)
	}
	if cfg.BaseURL != "http://localhost:8000" {
		t.Errorf("Expected BaseURL=http://localhost:8000, got %s", cfg.BaseURL)
	}
	if cfg.TransportMode != SSETransport {
		t.Errorf("Expected TransportMode=sse, got %s", cfg.TransportMode)
	}
	if cfg.Paths.SSE != "/sse" {
		t.Errorf("Expected Paths.SSE=/sse, got %s", cfg.Paths.SSE)
	}
	if cfg.Paths.Messages != "/messages" {
		t.Errorf("Expected Paths.Messages=/messages, got %s", cfg.Paths.Messages)
	}

	// Test default values
	if cfg.TimeoutSeconds != 15 {
		t.Errorf("Expected default TimeoutSeconds=15, got %d", cfg.TimeoutSeconds)
	}
	if cfg.Port != 8000 {
		t.Errorf("Expected default Port=8000, got %d", cfg.Port)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "Valid SSE config",
			config: Config{
				TransportMode: SSETransport,
				Paths: PathsConfig{
					SSE:      "/sse",
					Messages: "/messages",
				},
				BaseURL: "http://localhost:8000",
			},
			expectError: false,
		},
		{
			name: "Valid stdio config",
			config: Config{
				TransportMode: StdioTransport,
				Stdio: StdioConfig{
					Enabled:     true,
					UserCommand: "some-command",
				},
			},
			expectError: false,
		},
		{
			name: "Invalid stdio config - not enabled",
			config: Config{
				TransportMode: StdioTransport,
				Stdio: StdioConfig{
					Enabled:     false,
					UserCommand: "some-command",
				},
			},
			expectError: true,
		},
		{
			name: "Invalid stdio config - no command",
			config: Config{
				TransportMode: StdioTransport,
				Stdio: StdioConfig{
					Enabled:     true,
					UserCommand: "",
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectError && err == nil {
				t.Errorf("Expected validation error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestGetMCPPaths(t *testing.T) {
	cfg := Config{
		Paths: PathsConfig{
			SSE:            "/custom-sse",
			Messages:       "/custom-messages",
			StreamableHTTP: "/custom-streamable",
		},
	}

	paths := cfg.GetMCPPaths()
	if len(paths) != 3 {
		t.Errorf("Expected 3 MCP paths, got %d", len(paths))
	}
}

func TestBuildExecCommand(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedResult string
	}{
		{
			name: "Valid command",
			config: Config{
				Stdio: StdioConfig{
					UserCommand: "test-command",
				},
				Port:    8080,
				BaseURL: "http://example.com",
				Paths: PathsConfig{
					SSE:      "/sse-path",
					Messages: "/msgs",
				},
			},
			expectedResult: `npx -y supergateway --stdio "test-command" --port 8080 --baseUrl http://example.com --ssePath /sse-path --messagePath /msgs`,
		},
		{
			name: "Empty command",
			config: Config{
				Stdio: StdioConfig{
					UserCommand: "",
				},
			},
			expectedResult: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.BuildExecCommand()
			if result != tc.expectedResult {
				t.Errorf("Expected command=%s, got %s", tc.expectedResult, result)
			}
		})
	}
}
