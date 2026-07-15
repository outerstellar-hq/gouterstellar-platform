package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTOTPVerificationUsesAuthenticationRateLimit(t *testing.T) {
	assert.True(t, isAuthRoute("/auth/totp/verify"))
	assert.True(t, isAuthRoute("/api/v1/auth/totp/verify"))
	assert.False(t, isAuthRoute("/settings/totp/setup"))
}
