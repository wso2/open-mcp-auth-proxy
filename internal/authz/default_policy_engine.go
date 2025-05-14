package authz

import (
	"fmt"
	"net/http"
	"strings"

	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

type TokenClaims struct {
	Scopes []string
}

type DefaultPolicyEngine struct{}

// Evaluate and checks the token claims against one or more required scopes.
func (d *DefaultPolicyEngine) Evaluate(
	_ *http.Request,
	claims *TokenClaims,
	requiredScopes any,
) PolicyResult {

	logger.Info("Required scopes: %v", requiredScopes)

	var scopeStr string
	switch v := requiredScopes.(type) {
	case string:
		scopeStr = v
	case []string:
		scopeStr = strings.Join(v, " ")
	}

	if strings.TrimSpace(scopeStr) == "" {
		return PolicyResult{DecisionAllow, ""}
	}

	scopes := strings.FieldsFunc(scopeStr, func(r rune) bool {
		return r == ' ' || r == ','
	})
	required := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		if s = strings.TrimSpace(s); s != "" {
			required[s] = struct{}{}
		}
	}

	logger.Info("Token scopes: %v", claims.Scopes)
	for _, tokenScope := range claims.Scopes {
		if _, ok := required[tokenScope]; ok {
			return PolicyResult{DecisionAllow, ""}
		}
	}

	var list []string
	for s := range required {
		list = append(list, s)
	}
	return PolicyResult{
		DecisionDeny,
		fmt.Sprintf("missing required scope(s): %s", strings.Join(list, ", ")),
	}
}
