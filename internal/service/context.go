package service

import (
	"context"

	"github.com/rygel/gouterstellar-platform/internal/model"
)

// userCtxKey is the canonical context key for the authenticated user acting on
// a request. It is owned by the service layer so services can derive the actor
// (e.g. for notifications) without importing the web layer. The web layer's
// WithUser helper populates this key.
type userCtxKey struct{}

// ContextWithUser returns a derived context carrying the acting user. It is the
// service-layer counterpart the web layer calls when attaching the authenticated
// user to a request.
func ContextWithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, user)
}

// UserFromContext returns the acting user stored in ctx, or nil if none is set.
// Services should treat a nil result as "no authenticated actor" and skip
// actor-scoped side effects (e.g. notifications).
func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userCtxKey{}).(*model.User)
	return u
}

// ActorUserIDFromContext returns the string form of the acting user's ID, or an
// empty string when no authenticated user is present. Callers pass the result
// to user-scoped side effects (e.g. WebSocket refresh broadcasts): an empty
// string means "no known actor" and should be treated as a broadcast.
func ActorUserIDFromContext(ctx context.Context) string {
	user := UserFromContext(ctx)
	if user == nil {
		return ""
	}
	return user.ID.String()
}
