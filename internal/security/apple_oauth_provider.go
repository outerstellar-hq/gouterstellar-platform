package security

import "fmt"

type OAuthProvider interface {
	Name() string
	AuthorizationURL(state, redirectURI string) string
	ExchangeCode(code, state, redirectURI string) (*OAuthUserInfo, error)
}

type OAuthUserInfo struct {
	Subject     string
	Email       *string
	DisplayName *string
}

type OAuthException struct {
	Provider string
	Message  string
}

func (e *OAuthException) Error() string {
	return fmt.Sprintf("OAuth error (%s): %s", e.Provider, e.Message)
}

type AppleOAuthProvider struct{}

func NewAppleOAuthProvider() *AppleOAuthProvider {
	return &AppleOAuthProvider{}
}

func (p *AppleOAuthProvider) Name() string { return "apple" }

func (p *AppleOAuthProvider) AuthorizationURL(state, redirectURI string) string {
	return ""
}

func (p *AppleOAuthProvider) ExchangeCode(code, state, redirectURI string) (*OAuthUserInfo, error) {
	return nil, &OAuthException{
		Provider: "apple",
		Message:  "Apple OAuth is not yet implemented",
	}
}
