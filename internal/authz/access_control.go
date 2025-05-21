package authz

import (
	"net/http"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
)

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
	ValidateAccess(r *http.Request, claims *jwt.MapClaims, config *config.Config) AccessControlResult
}
