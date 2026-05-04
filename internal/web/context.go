package web

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rygel/gouterstellar-platform/internal/model"
)

type ContextKey string

const userContextKey ContextKey = "user"
const csrfContextKey ContextKey = "csrfToken"

func WithUser(r *http.Request, user *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

func UserFromRequest(r *http.Request) *model.User {
	u, _ := r.Context().Value(userContextKey).(*model.User)
	return u
}

func WithCSRFToken(r *http.Request, token string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), csrfContextKey, token))
}

func CSRFTokenFromRequest(r *http.Request) string {
	t, _ := r.Context().Value(csrfContextKey).(string)
	return t
}

func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKey("requestId")).(string); ok {
		return id
	}
	return uuid.New().String()[:8]
}
