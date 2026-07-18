package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
)

const sessionSubjectKey = "outerstellar.auth.subject"

var ErrUnauthenticated = errors.New("authentication required")

var ErrSessionIterationUnsupported = errors.New("session store does not support iteration")

// Principal is the application-neutral identity made available to HTTP
// handlers after a session has been resolved against the consumer's user
// authority.
type Principal struct {
	Subject string
	Roles   []string
	Claims  map[string]string
}

// HasRole performs an exact, case-sensitive role check.
func (p Principal) HasRole(role string) bool {
	return slices.Contains(p.Roles, role)
}

// PrincipalResolver loads current identity and authorization state. Consumers
// return ErrUnauthenticated for deleted or disabled subjects; other errors are
// treated as infrastructure failures and fail closed.
type PrincipalResolver interface {
	ResolvePrincipal(context.Context, string) (Principal, error)
}

// PrincipalResolverFunc adapts a function to PrincipalResolver.
type PrincipalResolverFunc func(context.Context, string) (Principal, error)

func (f PrincipalResolverFunc) ResolvePrincipal(ctx context.Context, subject string) (Principal, error) {
	return f(ctx, subject)
}

// SessionConfig centralizes session lifetime and cookie policy. Production
// cookies are Secure by default; local HTTP development must opt out explicitly.
type SessionConfig struct {
	CookieName           string
	Lifetime             time.Duration
	IdleTimeout          time.Duration
	PersistCookie        bool
	AllowInsecureCookies bool
	ErrorHandler         func(http.ResponseWriter, *http.Request, error)
}

// Sessions combines SCS lifecycle handling with current-principal resolution.
// Consumers supply a durable SCS store and their own user resolver.
type Sessions struct {
	manager  *scs.SessionManager
	resolver PrincipalResolver
}

// NewSessions constructs a session module. The store must be durable in
// multi-instance deployments; SCS supplies PostgreSQL, Redis and other stores.
func NewSessions(store scs.Store, resolver PrincipalResolver, config SessionConfig) (*Sessions, error) {
	if store == nil {
		return nil, errors.New("session store is required")
	}
	if resolver == nil {
		return nil, errors.New("principal resolver is required")
	}
	if strings.TrimSpace(config.CookieName) == "" {
		return nil, errors.New("session cookie name is required")
	}
	if config.Lifetime <= 0 {
		config.Lifetime = 12 * time.Hour
	}
	if config.IdleTimeout < 0 || config.IdleTimeout > config.Lifetime {
		return nil, errors.New("session idle timeout must be between zero and lifetime")
	}

	manager := scs.New()
	manager.Store = store
	manager.Lifetime = config.Lifetime
	manager.IdleTimeout = config.IdleTimeout
	manager.HashTokenInStore = true
	manager.Cookie.Name = config.CookieName
	manager.Cookie.HttpOnly = true
	manager.Cookie.Path = "/"
	manager.Cookie.SameSite = http.SameSiteStrictMode
	manager.Cookie.Secure = !config.AllowInsecureCookies
	manager.Cookie.Persist = config.PersistCookie
	if config.ErrorHandler != nil {
		manager.ErrorFunc = config.ErrorHandler
	}

	return &Sessions{manager: manager, resolver: resolver}, nil
}

// Middleware loads and commits the server-side session and resolves its
// subject to a current Principal before invoking the next handler.
func (s *Sessions) Middleware(next http.Handler) http.Handler {
	resolve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject := s.manager.GetString(r.Context(), sessionSubjectKey)
		if subject == "" {
			next.ServeHTTP(w, r)
			return
		}
		principal, err := s.resolver.ResolvePrincipal(r.Context(), subject)
		if errors.Is(err, ErrUnauthenticated) {
			if destroyErr := s.manager.Destroy(r.Context()); destroyErr != nil {
				s.manager.ErrorFunc(w, r, fmt.Errorf("destroy invalid session: %w", destroyErr))
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			s.manager.ErrorFunc(w, r, fmt.Errorf("resolve session principal: %w", err))
			return
		}
		if principal.Subject == "" || principal.Subject != subject {
			s.manager.ErrorFunc(w, r, errors.New("principal resolver returned a mismatched subject"))
			return
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
	return s.manager.LoadAndSave(resolve)
}

// SignIn rotates the session token before recording the authenticated subject,
// preventing session fixation across privilege changes.
func (s *Sessions) SignIn(ctx context.Context, subject string) error {
	if strings.TrimSpace(subject) == "" {
		return errors.New("session subject is required")
	}
	if err := s.manager.RenewToken(ctx); err != nil {
		return fmt.Errorf("renew session token: %w", err)
	}
	s.manager.Put(ctx, sessionSubjectKey, subject)
	return nil
}

// SignOut destroys the current server-side session and expires its cookie.
func (s *Sessions) SignOut(ctx context.Context) error {
	if err := s.manager.Destroy(ctx); err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	return nil
}

// RevokeSubject deletes every active session for subject. This is intended for
// password changes, account disablement, and other privilege resets. It
// deletes store keys directly because SCS iteration returns keys in their
// stored form, which may already be hashed.
func (s *Sessions) RevokeSubject(ctx context.Context, subject string) error {
	if strings.TrimSpace(subject) == "" {
		return errors.New("session subject is required")
	}
	all, err := s.allSessions(ctx)
	if err != nil {
		return err
	}
	for token, data := range all {
		_, values, decodeErr := s.manager.Codec.Decode(data)
		if decodeErr != nil {
			return fmt.Errorf("decode stored session: %w", decodeErr)
		}
		if values[sessionSubjectKey] != subject {
			continue
		}
		if deleteErr := s.deleteStoredSession(ctx, token); deleteErr != nil {
			return fmt.Errorf("revoke subject session: %w", deleteErr)
		}
	}
	return nil
}

// PrincipalFromContext returns the principal resolved by Middleware.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

// RequireAuthenticated rejects requests without a current principal.
func RequireAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFromContext(r.Context()); !ok {
			http.Error(w, ErrUnauthenticated.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole rejects authenticated principals that do not hold role.
func RequireRole(role string, next http.Handler) http.Handler {
	return RequireAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := PrincipalFromContext(r.Context())
		if !principal.HasRole(role) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

type principalContextKey struct{}

func withPrincipal(ctx context.Context, principal Principal) context.Context {
	principal.Roles = append([]string(nil), principal.Roles...)
	principal.Claims = cloneClaims(principal.Claims)
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func (s *Sessions) allSessions(ctx context.Context) (map[string][]byte, error) {
	if store, ok := s.manager.Store.(scs.IterableCtxStore); ok {
		all, err := store.AllCtx(ctx)
		if err != nil {
			return nil, fmt.Errorf("iterate sessions: %w", err)
		}
		return all, nil
	}
	if store, ok := s.manager.Store.(scs.IterableStore); ok {
		all, err := store.All()
		if err != nil {
			return nil, fmt.Errorf("iterate sessions: %w", err)
		}
		return all, nil
	}
	return nil, ErrSessionIterationUnsupported
}

func (s *Sessions) deleteStoredSession(ctx context.Context, token string) error {
	if store, ok := s.manager.Store.(scs.CtxStore); ok {
		return store.DeleteCtx(ctx, token)
	}
	return s.manager.Store.Delete(token)
}

func cloneClaims(claims map[string]string) map[string]string {
	if len(claims) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(claims))
	for name, value := range claims {
		cloned[name] = value
	}
	return cloned
}
