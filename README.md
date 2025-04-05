# Open MCP Auth Proxy

The Open MCP Auth Proxy is a lightweight proxy designed to sit in front of MCP servers and enforce authorization in compliance with the [Model Context Protocol authorization](https://spec.modelcontextprotocol.io/specification/2025-03-26/basic/authorization/) requirements. It intercepts incoming requests, validates tokens, and offloads authentication and authorization to an OAuth-compliant Identity Provider.

![image](https://github.com/user-attachments/assets/41cf6723-c488-4860-8640-8fec45006f92)

## **Setup and Installation**

### **Prerequisites**

* Go 1.20 or higher  
* A running MCP server (SSE transport supported)  
* An MCP client that supports MCP authorization 

### **Installation**

```bash
git clone https://github.com/wso2/open-mcp-auth-proxy  
cd open-mcp-auth-proxy  

go get github.com/golang-jwt/jwt/v4
go get gopkg.in/yaml.v2

go build -o openmcpauthproxy ./cmd/proxy
```

## Using Open MCP Auth Proxy

### Transport Modes

The Open MCP Auth Proxy supports two transport modes:

1. **SSE Mode (Default)**: For MCP servers that use Server-Sent Events transport
2. **stdio Mode**: For MCP servers that use stdio transport, which requires starting a subprocess

You can specify the transport mode in the `config.yaml` file:

```yaml
transport_mode: "sse"  # Options: "sse" or "stdio"
```

Or use the `--stdio` flag to override the configuration:

```bash
./openmcpauthproxy --stdio
```

### Configuration

The configuration uses a unified structure with common settings and transport-specific options:

```yaml
# Common configuration
listen_port: 8080
base_url: "http://localhost:8000"  # Base URL for the MCP server
port: 8000                         # Port for the MCP server

# Path configuration
paths:
  sse: "/sse"                      # SSE endpoint path
  messages: "/messages"            # Messages endpoint path

# Transport mode configuration
transport_mode: "sse"              # Options: "sse" or "stdio"

# stdio-specific configuration (used only when transport_mode is "stdio")
stdio:
  enabled: true
  user_command: "npx -y @modelcontextprotocol/server-github"
  work_dir: ""                     # Working directory (optional)
```

**Notes:**
- In SSE mode, the proxy connects to an external MCP server at the specified `base_url`
- In stdio mode, the proxy starts a subprocess using the `stdio.user_command` configuration
- Common settings like `base_url`, `port`, and `paths` are used for both transport modes

### Quick Start 

If you don't have an MCP server, follow the instructions given here to start your own MCP server for testing purposes.

1. Navigate to `resources` directory.
2. Initialize a virtual environment.

```bash
python3 -m venv .venv
```
3. Activate virtual environment.

```bash
source .venv/bin/activate
```

4. Install dependencies.

```
pip3 install -r requirements.txt
```

5. Start the server.

```bash
python3 echo_server.py
```

#### Configure the Auth Proxy

Update the necessary parameters in `config.yaml` as shown in the examples above.

#### Start the Auth Proxy

For the demo mode with pre-configured authentication:

```bash
./openmcpauthproxy --demo
```

For standard mode:

```bash
./openmcpauthproxy
```

For stdio mode:

```bash
./openmcpauthproxy --stdio
```

The `--demo` flag enables a demonstration mode with pre-configured authentication and authorization with a sandbox powered by [Asgardeo](https://asgardeo.io/).

#### Connect Using an MCP Client

You can use this fork of the [MCP Inspector](https://github.com/shashimalcse/inspector) to test the connection and try out the complete authorization flow. (This is a temporary fork with fixes for authentication [issues](https://github.com/modelcontextprotocol/typescript-sdk/issues/257) in the original implementation)

### Use with Asgardeo

Enable authorization for the MCP server through your own Asgardeo organization

1. [Register]([url](https://asgardeo.io/signup)) and create an organization in Asgardeo
2. Now, you need to authorize the OpenMCPAuthProxy to allow dynamically registering MCP Clients as applications in your organization. To do that,
   1. Create an [M2M application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/)  
         1. [Authorize this application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/#authorize-the-api-resources-for-the-app) to invoke "Application Management API" with the `internal_application_mgt_create` scope. 
             ![image](https://github.com/user-attachments/assets/0bd57cac-1904-48cc-b7aa-0530224bc41a)
         2. Note the **Client ID** and **Client secret** of this application. This is required by the auth proxy 

#### Configure the Auth Proxy

Create a configuration file config.yaml with the following parameters:

```yaml
# Common configuration
listen_port: 8080
base_url: "http://localhost:8000"  # Base URL for the MCP server

# Path configuration
paths:
  sse: "/sse"
  messages: "/messages"

# Transport mode
transport_mode: "sse"  # or "stdio"

asgardeo:                                     
  org_name: "<org_name>"                      # Your Asgardeo org name
  client_id: "<client_id>"                    # Client ID of the M2M app
  client_secret: "<client_secret>"            # Client secret of the M2M app
```

#### Start the Auth Proxy

```bash
./openmcpauthproxy --asgardeo
```

#### Integrating with existing OAuth Providers

 - [Auth0](docs/Auth0.md) - Enable authorization for the MCP server through your Auth0 organization.