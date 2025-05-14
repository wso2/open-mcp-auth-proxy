package authz

import (
	"net/http"
)

type TokenClaims struct {
	Scopes []string
}

type DefaulPolicyEngine struct{}

func (d *DefaulPolicyEngine) Evaluate(r *http.Request, claims *TokenClaims, requiredScope string) PolicyResult {
	for _, scope := range claims.Scopes {
		if scope == requiredScope {
			return PolicyResult{DecisionAllow, ""}
		}
	}
	return PolicyResult{DecisionDeny, "missing scope '" + requiredScope + "'"}
}
