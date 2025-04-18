# Open MCP Auth Proxy

A lightweight authorization proxy for Model Context Protocol (MCP) servers that enforces authorization according to the [MCP authorization specification](https://spec.modelcontextprotocol.io/specification/2025-03-26/basic/authorization/)

<a href="">[![🚀 Release](https://github.com/wso2/open-mcp-auth-proxy/actions/workflows/release.yml/badge.svg)](https://github.com/wso2/open-mcp-auth-proxy/actions/workflows/release.yml)</a>
<a href="">[![💬 Stackoverflow](https://img.shields.io/badge/Ask%20for%20help%20on-Stackoverflow-orange)](https://stackoverflow.com/questions/tagged/wso2is)</a>
<a href="">[![💬 Discord](https://img.shields.io/badge/Join%20us%20on-Discord-%23e01563.svg)](https://discord.gg/wso2)</a>
<a href="">[![🐦 Twitter](https://img.shields.io/twitter/follow/wso2.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=wso2)</a>
<a href="">[![📝 License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/wso2/product-is/blob/master/LICENSE)</a>

![Architecture Diagram](https://github.com/user-attachments/assets/41cf6723-c488-4860-8640-8fec45006f92)

## What it Does

Open MCP Auth Proxy sits between MCP clients and your MCP server to:

- Intercept incoming requests
- Validate authorization tokens
- Offload authentication and authorization to OAuth-compliant Identity Providers
- Support the MCP authorization protocol

## Quick Start

### Prerequisites

* Go 1.20 or higher
* A running MCP server
* An MCP client that supports MCP authorization

### Installation

```bash
git clone https://github.com/wso2/open-mcp-auth-proxy
cd open-mcp-auth-proxy
go get github.com/golang-jwt/jwt/v4 gopkg.in/yaml.v2
go build -o openmcpauthproxy ./cmd/proxy
```

### Basic Usage

1. The repository comes with a default `config.yaml` file that contains the basic configuration:

```yaml
listen_port: 8080
base_url: "http://localhost:8000"  # Your MCP server URL
paths:
  sse: "/sse"
  messages: "/messages/"
```

2. Start the proxy in demo mode (uses pre-configured authentication with Asgardeo sandbox):

```bash
./openmcpauthproxy --demo
```

3. Connect using an MCP client like [MCP Inspector](https://github.com/shashimalcse/inspector)(This is a temporary fork with fixes for authentication [issues](https://github.com/modelcontextprotocol/typescript-sdk/issues/257) in the original implementation)

## Identity Provider Integration

### Demo Mode

For quick testing, use the `--demo` flag which includes pre-configured authentication and authorization with an Asgardeo sandbox.

```bash
./openmcpauthproxy --demo
```

### Asgardeo Integration

To enable authorization through your own Asgardeo organization:

1. [Register](https://asgardeo.io/signup) and create an organization in Asgardeo
2. Create an [M2M application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/)
    1. [Authorize this application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/#authorize-the-api-resources-for-the-app) to invoke "Application Management API" with the `internal_application_mgt_create` scope
      ![image](https://github.com/user-attachments/assets/0bd57cac-1904-48cc-b7aa-0530224bc41a)
    2. Update the existing `config.yaml` with your Asgardeo details:

#### Configure the Auth Proxy

Create a configuration file config.yaml with the following parameters:

```yaml
base_url: "http://localhost:8000"  # URL of your MCP server  
listen_port: 8080                             # Address where the proxy will listen

asgardeo:                                     
  org_name: "<org_name>"                      # Your Asgardeo org name
  client_id: "<client_id>"                    # Client ID of the M2M app
  client_secret: "<client_secret>"            # Client secret of the M2M app
```

3. Start the proxy with Asgardeo integration:

```bash
./openmcpauthproxy --asgardeo
```

### Other OAuth Providers

- [Auth0](docs/integrations/Auth0.md)
- [Keycloak](docs/integrations/keycloak.md)

## Testing with an Example MCP Server

If you don't have an MCP server, you can use the included example:

1. Navigate to the `resources` directory
2. Set up a Python environment:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip3 install -r requirements.txt
```

3. Start the example server:

```bash
python3 echo_server.py
```

# Advanced Configuration

### Transport Modes

The proxy supports two transport modes:

- **SSE Mode (Default)**: For Server-Sent Events transport
- **stdio Mode**: For MCP servers that use stdio transport

When using stdio mode, the proxy:
- Starts an MCP server as a subprocess using the command specified in the configuration
- Communicates with the subprocess through standard input/output (stdio)
- **Note**: Any commands specified (like `npx` in the example below) must be installed on your system first

To use stdio mode:

```bash
./openmcpauthproxy --demo --stdio
```

#### Example: Running an MCP Server as a Subprocess

1. Configure stdio mode in your `config.yaml`:

```yaml
listen_port: 8080
base_url: "http://localhost:8000" 

stdio:
  enabled: true
  user_command: "npx -y @modelcontextprotocol/server-github"  # Example using a GitHub MCP server
  env:                           # Environment variables (optional)
    - "GITHUB_PERSONAL_ACCESS_TOKEN=gitPAT"

# CORS configuration
cors:
  allowed_origins:
    - "http://localhost:5173"  # Origin of your client application
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  allow_credentials: true

# Demo configuration for Asgardeo
demo:
  org_name: "openmcpauthdemo"
  client_id: "N0U9e_NNGr9mP_0fPnPfPI0a6twa"
  client_secret: "qFHfiBp5gNGAO9zV4YPnDofBzzfInatfUbHyPZvM0jka"    
```

2. Run the proxy with stdio mode:

```bash
./openmcpauthproxy --demo
```

The proxy will:
- Start the MCP server as a subprocess using the specified command
- Handle all authorization requirements
- Forward messages between clients and the server

### Complete Configuration Reference

```yaml
# Common configuration
listen_port: 8080
base_url: "http://localhost:8000"
port: 8000

# Path configuration
paths:
  sse: "/sse"
  messages: "/messages/"

# Transport mode
transport_mode: "sse"  # Options: "sse" or "stdio"

# stdio-specific configuration (used only in stdio mode)
stdio:
  enabled: true
  user_command: "npx -y @modelcontextprotocol/server-github"  # Command to start the MCP server (requires npx to be installed)
  work_dir: ""  # Optional working directory for the subprocess

# CORS configuration
cors:
  allowed_origins:
    - "http://localhost:5173"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  allow_credentials: true

# Demo configuration for Asgardeo
demo:
  org_name: "openmcpauthdemo"
  client_id: "N0U9e_NNGr9mP_0fPnPfPI0a6twa"
  client_secret: "qFHfiBp5gNGAO9zV4YPnDofBzzfInatfUbHyPZvM0jka"  

# Asgardeo configuration (used with --asgardeo flag)
asgardeo:
  org_name: "<org_name>"
  client_id: "<client_id>"
  client_secret: "<client_secret>"
```
