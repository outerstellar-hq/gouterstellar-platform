package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

type JwtConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Secret        string `mapstructure:"secret"`
	Issuer        string `mapstructure:"issuer"`
	ExpirySeconds int64  `mapstructure:"expiry_seconds"`
}

type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	StartTLS bool   `mapstructure:"starttls"`
}

type SegmentConfig struct {
	WriteKey string `mapstructure:"write_key"`
	Enabled  bool   `mapstructure:"enabled"`
}

// OAuthConfig groups third-party OAuth provider configuration. Each provider is
// independent; a provider is enabled when its ClientID/ClientSecret are set.
type OAuthConfig struct {
	Google GoogleOAuthConfig `mapstructure:"google"`
}

// GoogleOAuthConfig holds the credentials for the Google OAuth 2.0 / OpenID
// Connect provider.
type GoogleOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

type Config struct {
	Version               string        `mapstructure:"version"`
	Port                  int           `mapstructure:"port"`
	DatabaseURL           string        `mapstructure:"database_url"`
	DevDashboardEnabled   bool          `mapstructure:"dev_dashboard_enabled"`
	DevMode               bool          `mapstructure:"dev_mode"`
	SessionCookieSecure   bool          `mapstructure:"session_cookie_secure"`
	SessionTimeoutMinutes int           `mapstructure:"session_timeout_minutes"`
	CORSOrigins           string        `mapstructure:"cors_origins"`
	CSRFEnabled           bool          `mapstructure:"csrf_enabled"`
	AppBaseURL            string        `mapstructure:"app_base_url"`
	CSPPolicy             string        `mapstructure:"csp_policy"`
	PlatformMode          string        `mapstructure:"platform_mode"`
	JWT                   JwtConfig     `mapstructure:"jwt"`
	Email                 EmailConfig   `mapstructure:"email"`
	Segment               SegmentConfig `mapstructure:"segment"`
	OAuth                 OAuthConfig   `mapstructure:"oauth"`
}

func Load() *Config {
	v := viper.New()

	v.SetConfigName("application")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath(".")

	v.SetEnvPrefix("")
	v.AutomaticEnv()

	v.SetDefault("version", "dev")
	v.SetDefault("port", 8080)
	v.SetDefault("database_url", "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable")
	v.SetDefault("dev_dashboard_enabled", false)
	v.SetDefault("dev_mode", false)
	v.SetDefault("session_cookie_secure", false)
	v.SetDefault("session_timeout_minutes", 30)
	v.SetDefault("cors_origins", "*")
	v.SetDefault("csrf_enabled", true)
	v.SetDefault("app_base_url", "http://localhost:8080")
	v.SetDefault("platform_mode", "full")
	v.SetDefault("jwt.enabled", false)
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.issuer", "outerstellar")
	v.SetDefault("jwt.expiry_seconds", 86400)
	v.SetDefault("email.enabled", false)
	v.SetDefault("email.host", "localhost")
	v.SetDefault("email.port", 587)
	v.SetDefault("email.starttls", true)
	v.SetDefault("segment.enabled", false)
	v.SetDefault("segment.write_key", "")
	v.SetDefault("oauth.google.client_id", "")
	v.SetDefault("oauth.google.client_secret", "")
	v.SetDefault("oauth.google.redirect_uri", "")

	if err := v.ReadInConfig(); err != nil {
		slog.Warn("No config file found, using defaults and env vars", "error", err)
	}

	profile := v.GetString("APP_PROFILE")
	if profile != "" && profile != "default" {
		v.SetConfigName("application-" + profile)
		_ = v.MergeInConfig()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		slog.Error("Failed to unmarshal config", "error", err)
		panic(err)
	}
	return &cfg
}

// Validate checks the configuration for obvious errors.
// Returns a descriptive error if the config is invalid.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d (must be 1-65535)", c.Port)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database_url must not be empty")
	}
	if c.SessionTimeoutMinutes < 1 {
		return fmt.Errorf("session_timeout_minutes must be at least 1, got %d", c.SessionTimeoutMinutes)
	}
	switch c.PlatformMode {
	case "full", "extension-host", "headless", "":
		// valid
	default:
		return fmt.Errorf("invalid platform_mode %q (want full, extension-host, or headless)", c.PlatformMode)
	}
	if c.JWT.Enabled {
		if c.JWT.Secret == "" {
			return fmt.Errorf("jwt.enabled is true but jwt.secret is empty")
		}
		if c.JWT.ExpirySeconds < 1 {
			return fmt.Errorf("jwt.expiry_seconds must be positive, got %d", c.JWT.ExpirySeconds)
		}
	}
	if c.Email.Enabled {
		if c.Email.Host == "" {
			return fmt.Errorf("email.enabled is true but email.host is empty")
		}
		if c.Email.Port < 1 || c.Email.Port > 65535 {
			return fmt.Errorf("invalid email.port %d", c.Email.Port)
		}
		if c.Email.From == "" {
			return fmt.Errorf("email.enabled is true but email.from is empty")
		}
	}
	return nil
}
