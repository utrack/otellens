package exporter

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

const typeStr = "otellens"

// Config configures the otellens exporter behavior.
type Config struct {
	HTTPAddr              string        `mapstructure:"http_addr"`
	MaxConcurrentSessions int           `mapstructure:"max_concurrent_sessions"`
	DefaultSessionTimeout time.Duration `mapstructure:"default_session_timeout"`
	SessionBufferSize     int           `mapstructure:"session_buffer_size"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{
		HTTPAddr:              ":18080",
		MaxConcurrentSessions: 256,
		DefaultSessionTimeout: 30 * time.Second,
		SessionBufferSize:     64,
	}
}

// Validate ensures the config values are safe for runtime.
func (cfg *Config) Validate() error {
	if cfg.HTTPAddr == "" {
		return fmt.Errorf("http_addr must be set")
	}
	if cfg.MaxConcurrentSessions <= 0 {
		return fmt.Errorf("max_concurrent_sessions must be > 0")
	}
	if cfg.DefaultSessionTimeout <= 0 {
		return fmt.Errorf("default_session_timeout must be > 0")
	}
	if cfg.SessionBufferSize <= 0 {
		return fmt.Errorf("session_buffer_size must be > 0")
	}
	return nil
}
