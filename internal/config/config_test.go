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
	assert.Equal(t, 1440, cfg.SessionAbsoluteMinutes)
	assert.Equal(t, "outerstellar-dev-token-pepper", cfg.TokenPepper)
	assert.True(t, cfg.RegistrationEnabled)
	assert.Equal(t, int32(10), cfg.MaxFailedLoginAttempts)
	assert.Empty(t, cfg.TrustedProxies)
	assert.Equal(t, DefaultMaxRequestBodyBytes, cfg.MaxRequestBodyBytes)
	assert.Empty(t, cfg.StaticDir)
	assert.Equal(t, int64(900), cfg.LockoutDurationSeconds)
	assert.True(t, cfg.CSRFEnabled)
	assert.False(t, cfg.JWT.Enabled)
	assert.False(t, cfg.Email.Enabled)
	assert.False(t, cfg.Segment.Enabled)
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("SESSION_TIMEOUT_MINUTES", "60")
	os.Setenv("SESSION_ABSOLUTE_TIMEOUT_MINUTES", "720")
	os.Setenv("TOKEN_PEPPER", "test-token-pepper")
	os.Setenv("REGISTRATION_ENABLED", "false")
	os.Setenv("MAX_REQUEST_BODY_BYTES", "1048576")
	os.Setenv("TRUSTED_PROXIES", "10.0.0.1,10.0.0.2")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("SESSION_TIMEOUT_MINUTES")
	defer os.Unsetenv("SESSION_ABSOLUTE_TIMEOUT_MINUTES")
	defer os.Unsetenv("TOKEN_PEPPER")
	defer os.Unsetenv("REGISTRATION_ENABLED")
	defer os.Unsetenv("MAX_REQUEST_BODY_BYTES")
	defer os.Unsetenv("TRUSTED_PROXIES")

	cfg := Load()
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 60, cfg.SessionTimeoutMinutes)
	assert.Equal(t, 720, cfg.SessionAbsoluteMinutes)
	assert.Equal(t, "test-token-pepper", cfg.TokenPepper)
	assert.False(t, cfg.RegistrationEnabled)
	assert.Equal(t, int64(1048576), cfg.MaxRequestBodyBytes)
	assert.Equal(t, "10.0.0.1,10.0.0.2", cfg.TrustedProxies)
}

func TestLoadStarforgeConfigFromEnv(t *testing.T) {
	t.Setenv("STARFORGE_BASE_URL", "https://starforge.internal")
	t.Setenv("STARFORGE_CREDENTIAL", "server-only-token")

	cfg := Load()

	assert.Equal(t, "https://starforge.internal", cfg.Starforge.BaseURL)
	assert.Equal(t, "server-only-token", cfg.Starforge.Credential)
}

func TestLoadStaticDirectoryFromJavaCompatibleEnv(t *testing.T) {
	t.Setenv("ASSETS_DIR", "fallback-assets")
	t.Setenv("STATIC_DIR", "preferred-assets")

	cfg := Load()

	assert.Equal(t, "preferred-assets", cfg.StaticDir)
}

func TestLoadStaticDirectoryFromLegacyAssetsEnv(t *testing.T) {
	t.Setenv("STATIC_DIR", "")
	t.Setenv("ASSETS_DIR", "legacy-assets")

	cfg := Load()

	assert.Equal(t, "legacy-assets", cfg.StaticDir)
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
	valid := Config{
		Port:                   8080,
		DatabaseURL:            "postgres://localhost/db",
		SessionTimeoutMinutes:  30,
		SessionAbsoluteMinutes: 1440,
		MaxFailedLoginAttempts: 10,
		MaxRequestBodyBytes:    DefaultMaxRequestBodyBytes,
		LockoutDurationSeconds: 900,
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", nil, false},
		{"invalid port", func(c *Config) { c.Port = 0 }, true},
		{"empty database url", func(c *Config) { c.DatabaseURL = "" }, true},
		{"invalid absolute timeout", func(c *Config) { c.SessionAbsoluteMinutes = 0 }, true},
		{"jwt enabled no secret", func(c *Config) { c.JWT.Enabled = true }, true},
		{"email enabled no host", func(c *Config) { c.Email.Enabled = true }, true},
		{"valid platform_mode full", func(c *Config) { c.PlatformMode = "full" }, false},
		{"valid platform_mode extension-host", func(c *Config) { c.PlatformMode = "extension-host" }, false},
		{"valid platform_mode headless", func(c *Config) { c.PlatformMode = "headless" }, false},
		{"invalid platform_mode", func(c *Config) { c.PlatformMode = "bogus" }, true},
		{"invalid max failed attempts", func(c *Config) { c.MaxFailedLoginAttempts = 0 }, true},
		{"invalid max body size", func(c *Config) { c.MaxRequestBodyBytes = 0 }, true},
		{"invalid lockout duration", func(c *Config) { c.LockoutDurationSeconds = 0 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
