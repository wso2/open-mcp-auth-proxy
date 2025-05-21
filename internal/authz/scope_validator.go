package authz

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

type ScopeValidator struct{}

// Evaluate and checks the token claims against one or more required scopes.
func (d *ScopeValidator) ValidateAccess(
	r *http.Request,
	claims *jwt.MapClaims,
	config *config.Config,
) AccessControlResult {
    env, err := util.ParseRPCRequest(r)
    if err != nil {
        return AccessControlResult{DecisionDeny, "bad JSON-RPC request"}
    }
    requiredScopes := util.GetRequiredScopes(config, env.Method)
    if len(requiredScopes) == 0 {
        return AccessControlResult{DecisionAllow, ""}
    }

    required := make(map[string]struct{}, len(requiredScopes))
    for _, s := range requiredScopes {
        s = strings.TrimSpace(s)
        if s != "" {
            required[s] = struct{}{}
        }
    }

    var tokenScopes []string
    if claims, ok := (*claims)["scope"]; ok {
        switch v := claims.(type) {
        case string:
            tokenScopes = strings.Fields(v)
        case []interface{}:
            for _, x := range v {
                if s, ok := x.(string); ok && s != "" {
                    tokenScopes = append(tokenScopes, s)
                }
            }
        }
    }

	tokenScopeSet := make(map[string]struct{}, len(tokenScopes))
	for _, s := range tokenScopes {
		tokenScopeSet[s] = struct{}{}
	}

	var missing []string
	for s := range required {
		if _, ok := tokenScopeSet[s]; !ok {
			missing = append(missing, s)
		}
	}

	if len(missing) == 0 {
		return AccessControlResult{DecisionAllow, ""}
	}
	return AccessControlResult{
		DecisionDeny,
		fmt.Sprintf("missing required scope(s): %s", strings.Join(missing, ", ")),
	}
}
