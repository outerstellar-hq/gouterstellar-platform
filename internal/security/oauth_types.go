package security

import "fmt"

// OAuthProvider abstracts a third-party OAuth 2.0 / OpenID Connect provider so
// the OAuth handler can drive any backend behind a single interface.
type OAuthProvider interface {
	Name() string
	AuthorizationURL(state, redirectURI string) string
	ExchangeCode(code, state, redirectURI string) (*OAuthUserInfo, error)
}

// OAuthUserInfo is the normalized identity returned by a provider after a
// successful code exchange. Email and DisplayName are nil when the provider
// did not return them.
type OAuthUserInfo struct {
	Subject     string
	Email       *string
	DisplayName *string
}

// OAuthException is the error type providers return for provider-specific
// failures (bad code, endpoint error, missing fields).
type OAuthException struct {
	Provider string
	Message  string
}

func (e *OAuthException) Error() string {
	return fmt.Sprintf("OAuth error (%s): %s", e.Provider, e.Message)
}
