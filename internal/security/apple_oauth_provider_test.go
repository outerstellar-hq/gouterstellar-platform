package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppleOAuthProviderStaysOnHonestDisabledPath(t *testing.T) {
	t.Parallel()

	provider := NewAppleOAuthProvider()
	assert.Equal(t, "apple", provider.Name())
	assert.Equal(t, "/auth/oauth/apple/not-configured", provider.AuthorizationURL("state", "https://example.com/callback"))
	info, err := provider.ExchangeCode("code", "state", "https://example.com/callback")
	assert.Nil(t, info)
	var oauthErr *OAuthException
	require.ErrorAs(t, err, &oauthErr)
}
