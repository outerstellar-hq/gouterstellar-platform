package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type JwtConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Secret        string `yaml:"secret"`
	Issuer        string `yaml:"issuer"`
	ExpirySeconds int64  `yaml:"expiry_seconds"`
}

type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	StartTLS bool   `yaml:"starttls"`
}

type SegmentConfig struct {
	WriteKey string `yaml:"write_key"`
	Enabled  bool   `yaml:"enabled"`
}

// OAuthConfig groups third-party OAuth provider configuration. Each provider is
// independent; a provider is enabled when its ClientID/ClientSecret are set.
type OAuthConfig struct {
	Google GoogleOAuthConfig `yaml:"google"`
}

// GoogleOAuthConfig holds the credentials for the Google OAuth 2.0 / OpenID
// Connect provider.
type GoogleOAuthConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
}

type Config struct {
	Version               string        `yaml:"version"`
	Port                  int           `yaml:"port"`
	DatabaseURL           string        `yaml:"database_url"`
	DevDashboardEnabled   bool          `yaml:"dev_dashboard_enabled"`
	DevMode               bool          `yaml:"dev_mode"`
	SessionCookieSecure   bool          `yaml:"session_cookie_secure"`
	SessionTimeoutMinutes int           `yaml:"session_timeout_minutes"`
	CORSOrigins           string        `yaml:"cors_origins"`
	CSRFEnabled           bool          `yaml:"csrf_enabled"`
	AppBaseURL            string        `yaml:"app_base_url"`
	CSPPolicy             string        `yaml:"csp_policy"`
	PlatformMode          string        `yaml:"platform_mode"`
	JWT                   JwtConfig     `yaml:"jwt"`
	Email                 EmailConfig   `yaml:"email"`
	Segment               SegmentConfig `yaml:"segment"`
	OAuth                 OAuthConfig   `yaml:"oauth"`
}

// defaults returns a Config populated with the same default values the previous
// viper-based loader set via SetDefault. Profile YAML and env vars overlay these.
func defaults() *Config {
	return &Config{
		Version:               "dev",
		Port:                  8080,
		DatabaseURL:           "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable",
		DevDashboardEnabled:   false,
		DevMode:               false,
		SessionCookieSecure:   false,
		SessionTimeoutMinutes: 30,
		CORSOrigins:           "*",
		CSRFEnabled:           true,
		AppBaseURL:            "http://localhost:8080",
		PlatformMode:          "full",
		JWT: JwtConfig{
			Enabled:       false,
			Secret:        "",
			Issuer:        "outerstellar",
			ExpirySeconds: 86400,
		},
		Email: EmailConfig{
			Enabled:  false,
			Host:     "localhost",
			Port:     587,
			StartTLS: true,
		},
		Segment: SegmentConfig{
			Enabled:  false,
			WriteKey: "",
		},
		OAuth: OAuthConfig{
			Google: GoogleOAuthConfig{
				ClientID:     "",
				ClientSecret: "",
				RedirectURI:  "",
			},
		},
	}
}

// configPaths are the locations searched for config files, mirroring the old
// viper AddConfigPath calls. The first path that yields a file wins.
var configPaths = []string{"config", "."}

func Load() *Config {
	cfg := defaults()

	// Load base config, then overlay the profile selected by APP_PROFILE.
	loadYAML("application.yaml", cfg)

	if profile := os.Getenv("APP_PROFILE"); profile != "" && profile != "default" {
		loadYAML(fmt.Sprintf("application-%s.yaml", profile), cfg)
	}

	applyEnvOverrides(cfg)

	return cfg
}

// loadYAML reads name from each config path and unmarshals the first match onto
// cfg. yaml.Unmarshal only overwrites fields present in the file, so successive
// calls (base, then profile) layer correctly. A missing file is logged and
// ignored; a parse error is fatal because a half-parsed config would silently
// misconfigure the app.
func loadYAML(name string, cfg *Config) {
	for _, dir := range configPaths {
		path := dir + "/" + name
		if dir == "." {
			path = name
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Error("Failed to parse config file", "path", path, "error", err)
			panic(err)
		}
		return
	}
	slog.Warn("Config file not found, using defaults and env vars", "name", name)
}

// applyEnvOverrides reapplies the subset of env-var overrides the previous
// viper.AutomaticEnv mapping supported. Only the keys actually used in
// application.yaml, docker-compose, or tests are wired here; every other field
// keeps its YAML/default value.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("DEV_MODE"); v == "true" {
		cfg.DevMode = true
	}
	if v := os.Getenv("DEV_DASHBOARD_ENABLED"); v == "true" {
		cfg.DevDashboardEnabled = true
	}
	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("SESSION_TIMEOUT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SessionTimeoutMinutes = n
		}
	}
	if v := os.Getenv("SESSION_COOKIE_SECURE"); v == "true" {
		cfg.SessionCookieSecure = true
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = v
	}
	if v := os.Getenv("CSRF_ENABLED"); v == "false" {
		cfg.CSRFEnabled = false
	}
	if v := os.Getenv("APP_BASE_URL"); v != "" {
		cfg.AppBaseURL = v
	}
	if v := os.Getenv("PLATFORM_MODE"); v != "" {
		cfg.PlatformMode = v
	}
	if v := os.Getenv("CSP_POLICY"); v != "" {
		cfg.CSPPolicy = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("JWT_ENABLED"); v == "true" {
		cfg.JWT.Enabled = true
	}
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
	if c.OAuth.Google.ClientID != "" && c.OAuth.Google.ClientSecret == "" {
		return fmt.Errorf("oauth.google.client_id is set but oauth.google.client_secret is empty")
	}
	if c.OAuth.Google.ClientID == "" && c.OAuth.Google.ClientSecret != "" {
		return fmt.Errorf("oauth.google.client_secret is set but oauth.google.client_id is empty")
	}
	return nil
}
