// Package config handles configuration loading from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// Environment variable names read by Load.
const (
	envPort              = "PORT"
	envLogLevel          = "LOG_LEVEL"
	envDatabasePath      = "DATABASE_PATH"
	envSchedulerTickSecs = "SCHEDULER_TICK_SECS"
	envEncryptionKey     = "ENCRYPTION_KEY"
	envAuthUsername      = "AUTH_USERNAME"
	envAuthPassword      = "AUTH_PASSWORD"
)

// Config holds all application configuration values loaded from environment variables.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// LogLevel controls the minimum severity for log output.
	LogLevel zerolog.Level
	// DatabasePath is the file path to the SQLite database.
	DatabasePath string
	// SchedulerTickSecs is the interval between scheduler ticks in seconds.
	SchedulerTickSecs int
	// EncryptionKey is the required 32-byte AES-256-GCM key for encrypting API keys at rest.
	EncryptionKey []byte
	// AuthUsername and AuthPassword enable HTTP Basic Authentication when both are set.
	AuthUsername string
	// AuthPassword is the HTTP Basic Authentication password.
	AuthPassword string
}

// Load reads configuration from environment variables and returns a validated Config; parse errors are accumulated.
func Load() (*Config, error) {
	var p parser

	cfg := &Config{
		Port:              p.intVal(envPort, 9706),
		LogLevel:          p.logLevel(envLogLevel, zerolog.InfoLevel),
		DatabasePath:      envStr(envDatabasePath, "/config/huntarr2.db"),
		SchedulerTickSecs: p.intVal(envSchedulerTickSecs, 30),
		EncryptionKey:     p.encryptionKey(envEncryptionKey),
		AuthUsername:      envStr(envAuthUsername, ""),
		AuthPassword:      envStr(envAuthPassword, ""),
	}

	if err := errors.Join(p.errs...); err != nil {
		return nil, fmt.Errorf("parsing environment: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// IsDevelopment reports whether the application is running in a development context, approximated as debug log level.
func (c *Config) IsDevelopment() bool {
	return c.LogLevel == zerolog.DebugLevel
}

// AuthEnabled returns true when both AuthUsername and AuthPassword are configured.
func (c *Config) AuthEnabled() bool {
	return c.AuthUsername != "" && c.AuthPassword != ""
}

// validate checks all configuration constraints, returning an error joining every violation found.
func (c *Config) validate() error {
	var errs []error

	if c.DatabasePath == "" {
		errs = append(errs, fmt.Errorf("DATABASE_PATH is required"))
	}
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be between 1 and 65535"))
	}
	if c.SchedulerTickSecs < 5 {
		errs = append(errs, fmt.Errorf(
			"SCHEDULER_TICK_SECS must be at least 5, got %d",
			c.SchedulerTickSecs,
		))
	}
	hasUser := c.AuthUsername != ""
	hasPass := c.AuthPassword != ""
	if hasUser != hasPass {
		errs = append(errs, fmt.Errorf(
			"AUTH_USERNAME and AUTH_PASSWORD must both be set or both be empty",
		))
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("validating configuration: %w", err)
	}

	return nil
}

// EnvKeys returns a fresh slice of every environment variable that Load reads.
func EnvKeys() []string {
	return []string{
		envPort, envLogLevel, envDatabasePath,
		envSchedulerTickSecs, envEncryptionKey,
		envAuthUsername, envAuthPassword,
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
