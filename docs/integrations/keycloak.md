## Integrating Open MCP Auth Proxy with Keycloak

This guide walks you through configuring the Open MCP Auth Proxy to authenticate using Keycloak as the identity provider.

---

### Prerequisites

Before you begin, ensure you have the following:

- A running Keycloak instance
- Open MCP Auth Proxy installed and accessible

---

### Step 1: Configure Keycloak for Client Registration

Set up dynamic client registration in your Keycloak realm by following the [Keycloak client registration guide](https://www.keycloak.org/securing-apps/client-registration).

---

### Step 2: Configure Open MCP Auth Proxy

Update the `config.yaml` file in your Open MCP Auth Proxy setup using your Keycloak realm's [OIDC settings](https://www.keycloak.org/securing-apps/oidc-layers). Below is an example configuration:

```yaml
# Proxy server configuration
listen_port: 8081              # Port for the auth proxy
base_url: "http://localhost:8000" # Base URL of the MCP server
port: 8000                     # MCP server port

# Define path mappings
paths:
  sse: "/sse"
  messages: "/messages/"

# Set the transport mode
transport_mode: "sse"

# CORS settings
cors:
  allowed_origins:
    - "http://localhost:5173"  # Origin of your frontend/client app
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
    - "mcp-protocol-version"
  allow_credentials: true

# Keycloak endpoint path mappings
path_mapping:
  /token: /realms/master/protocol/openid-connect/token
  /register: /realms/master/clients-registrations/openid-connect

# Keycloak configuration block
default:
  base_url: "http://localhost:8080"
  jwks_url: "http://localhost:8080/realms/master/protocol/openid-connect/certs"
  path:
    /.well-known/oauth-authorization-server:
      response:
        issuer: "http://localhost:8080/realms/master"
        jwks_uri: "http://localhost:8080/realms/master/protocol/openid-connect/certs"
        authorization_endpoint: "http://localhost:8080/realms/master/protocol/openid-connect/auth"
        response_types_supported: 
          - "code"
        grant_types_supported:
          - "authorization_code"
          - "refresh_token"
        code_challenge_methods_supported:
          - "S256"
          - "plain"
    /token:
      addBodyParams:
        - name: "audience"
          value: "mcp_proxy"
```

### Step 3: Start the Auth Proxy

Launch the proxy with the updated Keycloak configuration:

```bash
./openmcpauthproxy
```

Once running, the proxy will handle authentication requests through your configured Keycloak realm.
