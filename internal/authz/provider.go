package authz

import "net/http"

// Provider is an interface describing how each auth provider
// will handle /.well-known/oauth-authorization-server and /register
type Provider interface {
	WellKnownHandler() http.HandlerFunc
	RegisterHandler() http.HandlerFunc
}
