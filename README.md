# open-mcp-auth-proxy

## Overview

OpenMCPAuthProxy is a security middleware that implements the Model Context Protocol (MCP) Authorization Specification (2025-03-26). It functions as a proxy between clients and MCP servers, providing robust authentication and authorization capabilities. The proxy intercepts incoming requests, validates authentication tokens, and forwards only authorized requests to the underlying MCP server, enhancing the security posture of your MCP deployment.

## Setup and Installation

### Prerequisites
- Go 1.20 or higher

### Installation
```bash
git clone https://github.com/wso2/open-mcp-auth-proxy
cd open-mcp-auth-proxy
go build -o openmcpauthproxy ./cmd/proxy
```

## Configuration

Create a configuration file `config.yaml` with the following parameters:

### demo mode configuration:

```yaml
mcp_server_base_url: "http://localhost:8000"  # URL of your MCP server
listen_address: ":8080"                       # Address where the proxy will listen
```

### asgardeo configuration:

```yaml 
mcp_server_base_url: "http://localhost:8000"  # URL of your MCP server
listen_address: ":8080"                       # Address where the proxy will listen

asgardeo:
    org_name: "your-org-name"
    client_id: "your-client-id"
    client_secret: "your-client-secret"
 ``` 


## Usage Example

### 1. Start the MCP Server

Create a file named `echo_server.py`:

```python
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("Echo")


@mcp.resource("echo://{message}")
def echo_resource(message: str) -> str:
    """Echo a message as a resource"""
    return f"Resource echo: {message}"


@mcp.tool()
def echo_tool(message: str) -> str:
    """Echo a message as a tool"""
    return f"Tool echo: {message}"


@mcp.prompt()
def echo_prompt(message: str) -> str:
    """Create an echo prompt"""
    return f"Please process this message: {message}"

if __name__ == "__main__":
    mcp.run(transport="sse")
```

Run the server:
```bash
python3 echo_server.py
```

### 2. Start the Auth Proxy

```bash
./openmcpauthproxy --demo
```

The `--demo` flag enables a demonstration mode with pre-configured authentication with [Asgardeo](https://asgardeo.io/) You can also use the `--asgardeo` flag to use your own Asgardeo configuration.

### 3. Connect Using an MCP Client

You can use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to test the connection:

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
