package security

// AppleOAuthProvider mirrors the Java provider's current availability: the
// authorization-code token exchange is not implemented, so configured users
// are sent to an explicit local status page instead of through a broken flow.
type AppleOAuthProvider struct{}

func NewAppleOAuthProvider() *AppleOAuthProvider { return &AppleOAuthProvider{} }

func (*AppleOAuthProvider) Name() string { return "apple" }

func (*AppleOAuthProvider) AuthorizationURL(_, _ string) string {
	return "/auth/oauth/apple/not-configured"
}

func (*AppleOAuthProvider) ExchangeCode(_, _, _ string) (*OAuthUserInfo, error) {
	return nil, &OAuthException{
		Provider: "apple",
		Message:  "Sign in with Apple is disabled until token exchange is implemented",
	}
}

var _ OAuthProvider = (*AppleOAuthProvider)(nil)
