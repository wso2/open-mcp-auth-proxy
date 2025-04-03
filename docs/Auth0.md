## Integrating with Auth0

This guide will help you configure Open MCP Auth Proxy to use Auth0 as your identity provider.

### Prerequisites

- An Auth0 organization (sign up here if you don't have one)
- Open MCP Auth Proxy installed

### Setting Up Auth0
1. [Enable Dynamic Client Registration](https://auth0.com/docs/get-started/applications/dynamic-client-registration)
    - Go to your Auth0 dashboard
    - Navigate to Settings > Advanced
    - Enable "OIDC Dynamic Application Registration"
2. Inorder to setup connections in dynamically created clients [promote Connections to Domain Level](https://auth0.com/docs/authenticate/identity-providers/promote-connections-to-domain-level)
3. Create an API in Auth0:
   - Go to your Auth0 dashboard
   - Navigate to Applications > APIs
   - Click on "Create API"
   - Set a Name (e.g., "MCP API")
   - Set an Identifier (e.g., "mcp_proxy")
   - Keep the default signing algorithm (RS256)
   - Click "Create"

### Configuring the Open MCP Auth Proxy

Update your `config.yaml` with Auth0 settings:

```yaml
# Basic proxy configuration
mcp_server_base_url: "http://localhost:8000"
listen_port: 8080
timeout_seconds: 10

# CORS configuration
cors:
  allowed_origins:
    - "http://localhost:5173"  # Your client application origin
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  allow_credentials: true

# Path mappings for Auth0 endpoints
path_mapping:
  /token: /oauth/token
  /register: /oidc/register

# Auth0 configuration
default:
  base_url: "https://YOUR_AUTH0_DOMAIN"  # e.g., https://dev-123456.us.auth0.com
  jwks_url: "https://YOUR_AUTH0_DOMAIN/.well-known/jwks.json"
  path:
    /.well-known/oauth-authorization-server:
      response:
        issuer: "https://YOUR_AUTH0_DOMAIN/"
        jwks_uri: "https://YOUR_AUTH0_DOMAIN/.well-known/jwks.json"
        authorization_endpoint: "https://YOUR_AUTH0_DOMAIN/authorize?audience=mcp_proxy" # Only if you created an API with this identifier
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
          value: "mcp_proxy"  # Only if you created an API with this identifier
```

Replace YOUR_AUTH0_DOMAIN with your Auth0 domain (e.g., dev-abc123.us.auth0.com).

## Starting the Proxy with Auth0 Integration
Start the proxy in default mode (which will use Auth0 based on your configuration):

```bash
./openmcpauthproxy
```
