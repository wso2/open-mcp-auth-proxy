package authz

import "net/http"

type Decision int

const (
    DecisionAllow Decision = iota
    DecisionDeny
)

type PolicyResult struct {
    Decision Decision
    Message  string
}

type PolicyEngine interface {
    Evaluate(r *http.Request, claims *TokenClaims, requiredScope string) PolicyResult
}
