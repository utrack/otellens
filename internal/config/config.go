package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains runtime options for otellens.
type Config struct {
	HTTPAddress        string
	MaxConcurrentViews int
	ShutdownTimeout    time.Duration
}

// Load builds config from environment variables.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:        getString("OTELLENS_HTTP_ADDR", ":18080"),
		MaxConcurrentViews: getInt("OTELLENS_MAX_CONCURRENT_SESSIONS", 256),
		ShutdownTimeout:    getDuration("OTELLENS_SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if cfg.MaxConcurrentViews <= 0 {
		return Config{}, fmt.Errorf("OTELLENS_MAX_CONCURRENT_SESSIONS must be > 0")
	}
	return cfg, nil
}

func getString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
