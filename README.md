# Open MCP Auth Proxy

The Open MCP Auth Proxy is a lightweight proxy designed to sit in front of MCP servers and enforce authorization in compliance with the [Model Context Protocol authorization](https://spec.modelcontextprotocol.io/specification/2025-03-26/basic/authorization/) requirements. It intercepts incoming requests, validates tokens, and offloads authentication and authorization to an OAuth-compliant Identity Provider.

![image](https://github.com/user-attachments/assets/fc728670-2fdb-4a63-bcc4-b9b6a6c8b4ba)

## **Setup and Installation**

### **Prerequisites**

* Go 1.20 or higher  
* A running MCP server (SSE transport supported)  
* An MCP client that supports MCP authorization 

### **Installation**

```bash
git clone https://github.com/wso2/open-mcp-auth-proxy  
cd open-mcp-auth-proxy  
go build -o openmcpauthproxy ./cmd/proxy
```

## Using Open MCP Auth Proxy

### Quick Start 

Allows you to just enable authorization for your MCP server with the preconfigured auth provider powered by Asgardeo.

If you don’t have an MCP server, follow the instructions given here to start your own MCP server for testing purposes.
1. Download [sample MCP server](resources/echo_server.py)
2. Run the server with
```bash
python3 echo_server.py
```

#### Configure the Auth Proxy

Create a configuration file config.yaml with the following parameters:

```yaml
mcp_server_base_url: "http://localhost:8000"  # URL of your MCP server  
listen_address: ":8080"                       # Address where the proxy will listen
```

#### Start the Auth Proxy

```bash
./openmcpauthproxy --demo
```

The `--demo` flag enables a demonstration mode with pre-configured authentication with a sandbox powered by [Asgardeo](https://asgardeo.io/).

#### Connect Using an MCP Client

You can use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to test the connection

### Use with Asgardeo

Enable authorization for the MCP server through your own Asgardeo organization

1. [Register]([url](https://asgardeo.io/signup)) and create an organization in Asgardeo
2. Create an [M2M application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/)  
   1. Authorize “Application Management API” with `internal_application_mgt_create` all scopes
      ![image](https://github.com/user-attachments/assets/0bd57cac-1904-48cc-b7aa-0530224bc41a)
   2. Note the client ID and client secret of this application. This is required by the auth proxy 

#### Configure the Auth Proxy

Create a configuration file config.yaml with the following parameters:

```yaml
mcp_server_base_url: "http://localhost:8000"  # URL of your MCP server  
listen_address: ":8080"                       # Address where the proxy will listen

asgardeo:                                     
  org_name: "<org_name>"                      # Your Asgardeo org name
  client_id: "<client_id>"                    # Client ID of the M2M app
  client_secret: "<client_secret>"            # Client secret of the M2M app
```

#### Start the Auth Proxy

```bash
./openmcpauthproxy --asgardeo
```

### Use with Auth0

Enable authorization for the MCP server through your Auth0 organization

**TODO**: Add instructions

[Enable dynamic application registration](https://auth0.com/docs/get-started/applications/dynamic-client-registration#enable-dynamic-client-registration) in your Auth0 organization

#### Configure the Auth Proxy

Create a configuration file config.yaml with the following parameters:

```yaml
mcp_server_base_url: "http://localhost:8000"     # URL of your MCP server  
listen_address: ":8080"                          # Address where the proxy will listen
```

**TODO**: Update the configs for Auth0.

#### Start the Auth Proxy

```bash
./openmcpauthproxy --auth0
```

### Use with a standard OAuth Server

Enable authorization for the MCP server with a compliant OAuth server

#### Configuration

Create a configuration file config.yaml with the following parameters:

```yaml
mcp_server_base_url: "http://localhost:8000"  # URL of your MCP server  
listen_address: ":8080"                       # Address where the proxy will listen
```
**TODO**: Update the configs for a standard OAuth Server.

#### Start the Auth Proxy

```bash
./openmcpauthproxy
```
