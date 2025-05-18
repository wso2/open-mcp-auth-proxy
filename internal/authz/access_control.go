package authz

import "net/http"

type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
)

type AccessControlResult struct {
	Decision Decision
	Message  string
}

type AccessControl interface {
	ValidateAccess(r *http.Request, claims *TokenClaims, requiredScopes any) AccessControlResult
}
