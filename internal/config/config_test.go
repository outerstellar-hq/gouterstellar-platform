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
	assert.False(t, cfg.SessionCookieSecure)
	assert.Equal(t, 30, cfg.SessionTimeoutMinutes)
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

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Port: 8080, DatabaseURL: "postgres://localhost/db", SessionTimeoutMinutes: 30}, false},
		{"invalid port", Config{Port: 0, DatabaseURL: "x", SessionTimeoutMinutes: 30}, true},
		{"empty database url", Config{Port: 8080, DatabaseURL: "", SessionTimeoutMinutes: 30}, true},
		{"jwt enabled no secret", Config{Port: 8080, DatabaseURL: "x", SessionTimeoutMinutes: 30, JWT: JwtConfig{Enabled: true}}, true},
		{"email enabled no host", Config{Port: 8080, DatabaseURL: "x", SessionTimeoutMinutes: 30, Email: EmailConfig{Enabled: true}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
