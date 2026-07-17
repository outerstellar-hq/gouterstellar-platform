package platform

import (
	"context"
	"net/http"
)

type requestContextKey struct{}

// RequestUser is the immutable, public projection of an authenticated
// platform user made available to compile-time extensions.
type RequestUser struct {
	ID       string
	Username string
	Role     string
	IsAdmin  bool
}

// RequestContext contains the minimal platform-owned request metadata an
// extension may consume. User is nil for anonymous public requests.
type RequestContext struct {
	User      *RequestUser
	CSRFToken string
	RequestID string
}

// RequestContextFrom returns the platform request context. Requests outside an
// assembled platform handler receive an explicit anonymous zero value.
func RequestContextFrom(r *http.Request) RequestContext {
	if r == nil {
		return RequestContext{}
	}
	requestContext, _ := r.Context().Value(requestContextKey{}).(RequestContext)
	return requestContext
}

// WithRequestContext lets a server-side host adapter attach an authenticated
// public context before invoking extension handlers. Values must originate
// from trusted host middleware, never directly from browser input.
func WithRequestContext(r *http.Request, requestContext RequestContext) *http.Request {
	return withRequestContext(r, requestContext)
}

func withRequestContext(r *http.Request, requestContext RequestContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestContextKey{}, requestContext))
}
