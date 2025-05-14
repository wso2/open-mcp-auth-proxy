package authz

import (
	"fmt"
	"net/http"
	"strings"
)

type TokenClaims struct {
	Scopes []string
}

type DefaultPolicyEngine struct{}

// Evaluate and checks the token claims against one or more required scopes.
func (d *DefaultPolicyEngine) Evaluate(
	_ *http.Request,
	claims *TokenClaims,
	requiredScope string,
) PolicyResult {
	if strings.TrimSpace(requiredScope) == "" {
		return PolicyResult{DecisionAllow, ""}
	}

	raw := strings.FieldsFunc(requiredScope, func(r rune) bool {
		return r == ' ' || r == ','
	})
	want := make(map[string]struct{}, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			want[s] = struct{}{}
		}
	}

	for _, have := range claims.Scopes {
		if _, ok := want[have]; ok {
			return PolicyResult{DecisionAllow, ""}
		}
	}

	var list []string
	for s := range want {
		list = append(list, s)
	}
	return PolicyResult{
		DecisionDeny,
		fmt.Sprintf("missing required scope(s): %s", strings.Join(list, ", ")),
	}
}
