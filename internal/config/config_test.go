package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	cfg := Load()
	assert.Equal(t, 8080, cfg.Port)
	assert.Contains(t, cfg.DatabaseURL, "postgres://")
	assert.True(t, cfg.SessionCookieSecure)
	assert.Equal(t, "https://localhost:8080", cfg.CORSOrigins)
	assert.Equal(t, 30, cfg.SessionTimeoutMinutes)
	assert.Equal(t, int32(10), cfg.MaxFailedLoginAttempts)
	assert.Equal(t, int64(900), cfg.LockoutDurationSeconds)
	assert.True(t, cfg.CSRFEnabled)
	assert.False(t, cfg.JWT.Enabled)
	assert.False(t, cfg.Email.Enabled)
	assert.False(t, cfg.Segment.Enabled)
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("SESSION_TIMEOUT_MINUTES", "60")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("SESSION_TIMEOUT_MINUTES")

	cfg := Load()
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 60, cfg.SessionTimeoutMinutes)
}

func TestJwtConfigDefaults(t *testing.T) {
	cfg := Load()
	assert.Equal(t, "outerstellar", cfg.JWT.Issuer)
	assert.Equal(t, int64(86400), cfg.JWT.ExpirySeconds)
}

func TestEmailConfigDefaults(t *testing.T) {
	cfg := Load()
	assert.Equal(t, "localhost", cfg.Email.Host)
	assert.Equal(t, 587, cfg.Email.Port)
	assert.True(t, cfg.Email.StartTLS)
}
