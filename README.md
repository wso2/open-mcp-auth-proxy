# Open MCP Auth Proxy

A lightweight authorization proxy for Model Context Protocol (MCP) servers that enforces authorization according to the [MCP authorization specification](https://spec.modelcontextprotocol.io/specification/2025-03-26/basic/authorization/)

<a href="">[![🚀 Release](https://github.com/wso2/open-mcp-auth-proxy/actions/workflows/release.yml/badge.svg)](https://github.com/wso2/open-mcp-auth-proxy/actions/workflows/release.yml)</a>
<a href="">[![💬 Stackoverflow](https://img.shields.io/badge/Ask%20for%20help%20on-Stackoverflow-orange)](https://stackoverflow.com/questions/tagged/wso2is)</a>
<a href="">[![💬 Discord](https://img.shields.io/badge/Join%20us%20on-Discord-%23e01563.svg)](https://discord.gg/wso2)</a>
<a href="">[![🐦 Twitter](https://img.shields.io/twitter/follow/wso2.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=wso2)</a>
<a href="">[![📝 License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/wso2/product-is/blob/master/LICENSE)</a>

![Architecture Diagram](https://github.com/user-attachments/assets/41cf6723-c488-4860-8640-8fec45006f92)

## 🚀 Features

- **Dynamic Authorization**: based on MCP Authorization Specification.
- **JWT Validation**: Validates the token’s signature, checks the `audience` claim, and enforces scope requirements.
- **Identity Provider Integration**: Supports integrating any OAuth/OIDC provider such as Asgardeo, Auth0, Keycloak, etc.
- **Protocol Version Negotiation**: via `MCP-Protocol-Version` header.
- **Flexible Transport Modes**: Supports STDIO, SSE and streamable HTTP transport options.

## 🛠️ Quick Start

> **Prerequisites**
>
> * A running MCP server (Use the [example MCP server](resources/README.md) if you don't have an MCP server already)
> * An MCP client that supports MCP authorization specification

1. Download the latest release from [Github releases](https://github.com/wso2/open-mcp-auth-proxy/releases/latest).

2. Start the proxy in demo mode (uses pre-configured authentication with Asgardeo sandbox):

- Linux/macOS:

```bash
./openmcpauthproxy --demo
```

- Windows:

```powershell
.\openmcpauthproxy.exe --demo
```

3. Connect using an MCP client like [MCP Inspector](https://github.com/modelcontextprotocol/inspector).

## 🔒 Integrate an Identity Provider

### Asgardeo

To enable authorization through your Asgardeo organization:

1. [Register](https://asgardeo.io/signup) and create an organization in Asgardeo
2. Create an [M2M application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/)
3. [Authorize this application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/#authorize-the-api-resources-for-the-app) to invoke "Application Management API" with the `internal_application_mgt_create` scope
      ![image](https://github.com/user-attachments/assets/0bd57cac-1904-48cc-b7aa-0530224bc41a)
   
4. Update `config.yaml` with the following parameters.

```yaml
base_url: "http://localhost:8000"                              # URL of your MCP server  
listen_port: 8080                                              # Address where the proxy will listen

resource_identifier: "http://localhost:8080"                 # Proxy server URL
scopes_supported:                                            # Scopes required to defined for the MCP server
- "read:tools"
- "read:resources"
audience: "<audience_value>"                                 # Access token audience
authorization_servers:                                       # Authorization server issuer identifier(s)
- "https://api.asgardeo.io/t/acme"
jwks_uri: "https://api.asgardeo.io/t/acme/oauth2/jwks"       # JWKS URL
```

5. Start the proxy with Asgardeo integration:

```bash
./openmcpauthproxy --asgardeo
```

### Other OAuth Providers

- [Auth0](docs/integrations/Auth0.md)
- [Keycloak](docs/integrations/keycloak.md)

## Transport Modes

### **STDIO Mode**

When using stdio mode, the proxy:
- Starts an MCP server as a subprocess using the command specified in the configuration
- Communicates with the subprocess through standard input/output (stdio)

> **Note**: Any commands specified (like `npx` in the example below) must be installed on your system first

1. Configure stdio mode in your `config.yaml`:

```yaml
stdio:
  enabled: true
  user_command: "npx -y @modelcontextprotocol/server-github"  # Example using a GitHub MCP server
  env:                           # Environment variables (optional)
    - "GITHUB_PERSONAL_ACCESS_TOKEN=gitPAT"    
```

2. Run the proxy with stdio mode:

```bash
./openmcpauthproxy --demo
```

- **SSE Mode (Default)**: For Server-Sent Events transport
- **Streamable HTTP Mode**: For Streamable HTTP transport

## Available Command Line Options

```bash
# Start in demo mode (using Asgardeo sandbox)
./openmcpauthproxy --demo

# Start with your own Asgardeo organization
./openmcpauthproxy --asgardeo

# Use stdio transport mode instead of SSE
./openmcpauthproxy --demo --stdio

# Enable debug logging
./openmcpauthproxy --demo --debug

# Show all available options
./openmcpauthproxy --help
```

## Contributing

We appreciate your contributions, whether it is improving documentation, adding new features, or fixing bugs. To get started, please refer to our [contributing guide](CONTRIBUTING.md).
