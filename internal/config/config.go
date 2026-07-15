package config

import (
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

type Config struct {
	Version                string        `mapstructure:"version"`
	Port                   int           `mapstructure:"port"`
	DatabaseURL            string        `mapstructure:"database_url"`
	DevDashboardEnabled    bool          `mapstructure:"dev_dashboard_enabled"`
	DevMode                bool          `mapstructure:"dev_mode"`
	SessionCookieSecure    bool          `mapstructure:"session_cookie_secure"`
	SessionTimeoutMinutes  int           `mapstructure:"session_timeout_minutes"`
	MaxFailedLoginAttempts int32         `mapstructure:"max_failed_login_attempts"`
	LockoutDurationSeconds int64         `mapstructure:"lockout_duration_seconds"`
	CORSOrigins            string        `mapstructure:"cors_origins"`
	CSRFEnabled            bool          `mapstructure:"csrf_enabled"`
	AppBaseURL             string        `mapstructure:"app_base_url"`
	CSPPolicy              string        `mapstructure:"csp_policy"`
	MetricsToken           string        `mapstructure:"metrics_token"`
	JWT                    JwtConfig     `mapstructure:"jwt"`
	Email                  EmailConfig   `mapstructure:"email"`
	Segment                SegmentConfig `mapstructure:"segment"`
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
	v.SetDefault("database_url", "postgres://localhost:5432/outerstellar?sslmode=require")
	v.SetDefault("dev_dashboard_enabled", false)
	v.SetDefault("dev_mode", false)
	v.SetDefault("session_cookie_secure", true)
	v.SetDefault("session_timeout_minutes", 30)
	v.SetDefault("max_failed_login_attempts", 10)
	v.SetDefault("lockout_duration_seconds", 900)
	v.SetDefault("cors_origins", "https://localhost:8080")
	v.SetDefault("csrf_enabled", true)
	v.SetDefault("app_base_url", "https://localhost:8080")
	v.SetDefault("metrics_token", "")
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
	if cfg.MaxFailedLoginAttempts <= 0 {
		panic("max_failed_login_attempts must be greater than zero")
	}
	if cfg.LockoutDurationSeconds <= 0 {
		panic("lockout_duration_seconds must be greater than zero")
	}
	return &cfg
}
