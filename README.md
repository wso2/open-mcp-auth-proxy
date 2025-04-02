# Open MCP Auth Proxy

The Model Context Protocol (MCP) specification necessitates that MCP servers use OAuth-based authorization. However, directly implementing OAuth in the MCP servers adds complexity, requires specialized knowledge, and shifts focus away from the server's core functionality.

The OpenMCPAuth Proxy, a lightweight proxy, sits in front of MCP servers to secure access by enforcing OAuth standards. Concealing the implementation details, it gives the MCP server the inherent ability to function as an authorization provider.

The proxy intercepts incoming requests and validates Authorization: Bearer tokens, but delegates authentication (user login, consent, token issuance) to an Auth Provider, thereby decoupling authentication logic from the core MCP service. 

![image](https://github.com/user-attachments/assets/fc728670-2fdb-4a63-bcc4-b9b6a6c8b4ba)

## **Setup and Installation**

### **Prerequisites**

* Go 1.20 or higher  
* A running MCP server (SSE transport supported)  
* An MCP client that supports MCP authorization 

### **Installation**

```
git clone https://github.com/wso2/open-mcp-auth-proxy  
cd open-mcp-auth-proxy  
go build \-o openmcpauthproxy ./cmd/proxy
```

## Using Open MCP Auth Proxy

### Quick start with demowear 

Allows you to just enable authorization for your MCP server with the preconfigured auth provider powered by Asgardeo.

If you don’t have an MCP server, as mentioned in the prerequisites, follow the instructions given here to start your own MCP server for sandbox purposes. 

#### Configuration

Create a configuration file config.yaml with the following parameters:

```
mcp\_server\_base\_url: "http://localhost:8000"  \# URL of your MCP server  
listen\_address: ":8080"                       \# Address where the proxy will listen
```

#### Start the Auth Proxy

`./openmcpauthproxy \--demo

The \--demo flag enables a demonstration mode with pre-configured authentication with a sandbox powered by [Asgardeo](https://asgardeo.io/).

#### Connect Using an MCP Client

You can use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to test the connection:

### Use with Asgardeo

Enable authorization for the MCP server through your own Asgardeo organization

1. Register for Asgaradeo and create an organization for you  
2. Create an [M2M application](https://wso2.com/asgardeo/docs/guides/applications/register-machine-to-machine-app/)  
   1. Enable client credential grant   
   2. Authorize “Application Management API” internal\_application\_mgt\_create all scopes![][image2]

   3. Note the client ID and client secret of this application. This is required by the auth proxy 

#### Configuration

Create a configuration file config.yaml with the following parameters:

```
mcp\_server\_base\_url: "http://localhost:8000"  \# URL of your MCP server  
listen\_address: ":8080"                       \# Address where the proxy will listen
```

TODO: Update the configs for asgardeo.

#### Start the Auth Proxy

```./openmcpauthproxy \--asgardeo

### Use with Auth0

Enable authorization for the MCP server through your Auth0 organization

TODO: Add instructions

[Enable dynamic application registration](https://auth0.com/docs/get-started/applications/dynamic-client-registration#enable-dynamic-client-registration) in your Auth0 organization

#### Configuration

Create a configuration file config.yaml with the following parameters:

```mcp\_server\_base\_url: "http://localhost:8000"  \# URL of your MCP server  
listen\_address: ":8080"                       \# Address where the proxy will listen```
`
TODO: Update the configs for Auth0.

#### Start the Auth Proxy

```./openmcpauthproxy \--auth0

### Use with a standard OAuth Server

Enable authorization for the MCP server with a compliant OAuth server

TODO:Add instructions

#### Configuration

Create a configuration file config.yaml with the following parameters:

```mcp\_server\_base\_url: "http://localhost:8000"  \# URL of your MCP server  
listen\_address: ":8080"                       \# Address where the proxy will listen  
TODO: Update the configs for a standard OAuth Server.```

#### Start the Auth Proxy

```./openmcpauthproxy
