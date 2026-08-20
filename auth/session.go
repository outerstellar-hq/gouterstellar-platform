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

const (
	sessionSubjectKey         = "outerstellar.auth.subject"
	sessionSecurityVersionKey = "outerstellar.auth.security_version"
)

var ErrUnauthenticated = errors.New("authentication required")

// SecurityVersion is an opaque application-owned credential and authorization
// epoch. Applications rotate it atomically with security-sensitive changes.
type SecurityVersion string

// SessionIdentity is captured when credentials are verified and stored in the
// new session. SecurityVersion must be the value observed during verification,
// not a value re-read later during sign-in.
type SessionIdentity struct {
	Subject         string
	SecurityVersion SecurityVersion
}

// Principal is the immutable application-neutral identity returned by the
// consumer's user authority for every authenticated request.
type Principal struct {
	Subject         string
	SecurityVersion SecurityVersion
	roles           []string
	claims          map[string]string
}

// NewPrincipal constructs an immutable principal and copies caller-owned
// roles and claims before the value crosses the session seam.
func NewPrincipal(subject string, securityVersion SecurityVersion, roles []string, claims map[string]string) Principal {
	return Principal{
		Subject:         subject,
		SecurityVersion: securityVersion,
		roles:           cloneRoles(roles),
		claims:          cloneClaims(claims),
	}
}

// HasRole performs an exact, case-sensitive role check.
func (p Principal) HasRole(role string) bool {
	return slices.Contains(p.roles, role)
}

// Roles returns an independent copy of the principal's roles.
func (p Principal) Roles() []string {
	return cloneRoles(p.roles)
}

// Claims returns an independent copy of the principal's claims.
func (p Principal) Claims() map[string]string {
	return cloneClaims(p.claims)
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
	SameSite             http.SameSite
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
	if config.Lifetime < 0 {
		return nil, errors.New("session lifetime cannot be negative")
	}
	if config.Lifetime == 0 {
		config.Lifetime = 12 * time.Hour
	}
	if config.IdleTimeout < 0 || config.IdleTimeout > config.Lifetime {
		return nil, errors.New("session idle timeout must be between zero and lifetime")
	}
	if config.SameSite == 0 || config.SameSite == http.SameSiteDefaultMode {
		config.SameSite = http.SameSiteLaxMode
	}
	if config.SameSite != http.SameSiteLaxMode &&
		config.SameSite != http.SameSiteStrictMode &&
		config.SameSite != http.SameSiteNoneMode {
		return nil, errors.New("session SameSite must be Lax, Strict, or None")
	}
	if config.SameSite == http.SameSiteNoneMode && config.AllowInsecureCookies {
		return nil, errors.New("SameSite=None requires secure session cookies")
	}

	manager := scs.New()
	manager.Store = store
	manager.Lifetime = config.Lifetime
	manager.IdleTimeout = config.IdleTimeout
	manager.HashTokenInStore = true
	manager.Cookie.Name = config.CookieName
	manager.Cookie.HttpOnly = true
	manager.Cookie.Path = "/"
	manager.Cookie.SameSite = config.SameSite
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
		securityVersion := SecurityVersion(s.manager.GetString(r.Context(), sessionSecurityVersionKey))
		if securityVersion == "" {
			if !s.destroyInvalidSession(w, r, errors.New("session security version is missing")) {
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		principal, err := s.resolver.ResolvePrincipal(r.Context(), subject)
		if errors.Is(err, ErrUnauthenticated) {
			if !s.destroyInvalidSession(w, r, err) {
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
		if principal.SecurityVersion == "" || principal.SecurityVersion != securityVersion {
			if !s.destroyInvalidSession(w, r, errors.New("session security version is stale")) {
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
	return s.manager.LoadAndSave(resolve)
}

// SignIn rotates the session token before recording the identity captured by
// credential verification. The security version makes stale sessions fail
// closed even when they are committed concurrently with a version rotation.
func (s *Sessions) SignIn(ctx context.Context, identity SessionIdentity) error {
	if strings.TrimSpace(identity.Subject) == "" {
		return errors.New("session subject is required")
	}
	if identity.SecurityVersion == "" {
		return errors.New("session security version is required")
	}
	if err := s.manager.RenewToken(ctx); err != nil {
		return fmt.Errorf("renew session token: %w", err)
	}
	s.manager.Put(ctx, sessionSubjectKey, identity.Subject)
	s.manager.Put(ctx, sessionSecurityVersionKey, string(identity.SecurityVersion))
	return nil
}

// SignOut destroys the current server-side session and expires its cookie.
func (s *Sessions) SignOut(ctx context.Context) error {
	if err := s.manager.Destroy(ctx); err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	return nil
}

// Put stores an application-scoped string in the session, persisted by the
// Middleware response commit. Callers own their key space; the platform
// reserves its internal keys, so applications should prefix theirs (for
// example "mfaPendingSubject").
func (s *Sessions) Put(ctx context.Context, key, value string) {
	s.manager.Put(ctx, key, value)
}

// GetString reads an application-scoped session string; "" when unset.
func (s *Sessions) GetString(ctx context.Context, key string) string {
	return s.manager.GetString(ctx, key)
}

// Remove deletes an application-scoped session value.
func (s *Sessions) Remove(ctx context.Context, key string) {
	s.manager.Remove(ctx, key)
}

// PrincipalFromContext returns the immutable principal resolved by Middleware.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok {
		return Principal{}, false
	}
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
	principal.roles = cloneRoles(principal.roles)
	principal.claims = cloneClaims(principal.claims)
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func (s *Sessions) destroyInvalidSession(w http.ResponseWriter, r *http.Request, cause error) bool {
	if err := s.manager.Destroy(r.Context()); err != nil {
		s.manager.ErrorFunc(w, r, fmt.Errorf("destroy invalid session after %v: %w", cause, err))
		return false
	}
	return true
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

func cloneRoles(roles []string) []string {
	return append([]string(nil), roles...)
}
