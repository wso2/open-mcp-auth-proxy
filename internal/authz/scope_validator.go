package authz

import (
	"fmt"
	"net/http"
	"strings"
)

type TokenClaims struct {
	Scopes []string
}

type ScopeValidator struct{}

// Evaluate and checks the token claims against one or more required scopes.
func (d *ScopeValidator) ValidateAccess(
	_ *http.Request,
	claims *TokenClaims,
	requiredScopes any,
) AccessControlResult {
	var scopeStr string
	switch v := requiredScopes.(type) {
	case string:
		scopeStr = v
	case []string:
		scopeStr = strings.Join(v, " ")
	}

	if strings.TrimSpace(scopeStr) == "" {
		return AccessControlResult{DecisionAllow, ""}
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

	for _, tokenScope := range claims.Scopes {
		if _, ok := required[tokenScope]; ok {
			return AccessControlResult{DecisionAllow, ""}
		}
	}

	var list []string
	for s := range required {
		list = append(list, s)
	}
	return AccessControlResult{
		DecisionDeny,
		fmt.Sprintf("missing required scope(s): %s", strings.Join(list, ", ")),
	}
}
