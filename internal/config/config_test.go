package config

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// testHexKey returns a valid 64-character hex string representing 32 bytes.
func testHexKey() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

// testBase64Key returns a valid base64 encoding of 32 bytes.
func testBase64Key() string {
	raw, _ := hex.DecodeString(testHexKey())
	return base64.StdEncoding.EncodeToString(raw)
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		check   func(t *testing.T, cfg *Config)
		wantErr string
	}{
		{
			name: "defaults with required vars",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Port != 9706 {
					t.Errorf("Port = %d, want 9706", cfg.Port)
				}
				if cfg.LogLevel != zerolog.InfoLevel {
					t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
				}
				if cfg.DatabasePath != "/config/huntarr2.db" {
					t.Errorf("DatabasePath = %q, want /config/huntarr2.db",
						cfg.DatabasePath)
				}
				if cfg.SchedulerTickSecs != 30 {
					t.Errorf("SchedulerTickSecs = %d, want 30",
						cfg.SchedulerTickSecs)
				}
				if cfg.IsDevelopment() {
					t.Error("IsDevelopment() = true, want false")
				}
				if len(cfg.EncryptionKey) != 32 {
					t.Errorf("EncryptionKey length = %d, want 32",
						len(cfg.EncryptionKey))
				}
			},
		},
		{
			name: "all overrides",
			env: map[string]string{
				"DATABASE_PATH":       "/data/test.db",
				"PORT":                "8080",
				"LOG_LEVEL":           "debug",
				"SCHEDULER_TICK_SECS": "15",
				"ENCRYPTION_KEY":      testHexKey(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Port != 8080 {
					t.Errorf("Port = %d, want 8080", cfg.Port)
				}
				if cfg.LogLevel != zerolog.DebugLevel {
					t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
				}
				if cfg.DatabasePath != "/data/test.db" {
					t.Errorf("DatabasePath = %q, want /data/test.db",
						cfg.DatabasePath)
				}
				if cfg.SchedulerTickSecs != 15 {
					t.Errorf("SchedulerTickSecs = %d, want 15",
						cfg.SchedulerTickSecs)
				}
				if !cfg.IsDevelopment() {
					t.Error("IsDevelopment() = false, want true")
				}
			},
		},
		{
			name: "empty DATABASE_PATH uses default",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.DatabasePath != "/config/huntarr2.db" {
					t.Errorf("DatabasePath = %q, want /config/huntarr2.db",
						cfg.DatabasePath)
				}
			},
		},
		{
			name: "invalid port",
			env: map[string]string{
				"PORT":           "abc",
				"ENCRYPTION_KEY": testHexKey(),
			},
			wantErr: "not a valid integer",
		},
		{
			name: "port out of range",
			env: map[string]string{
				"PORT":           "99999",
				"ENCRYPTION_KEY": testHexKey(),
			},
			wantErr: "PORT must be between 1 and 65535",
		},
		{
			name: "scheduler tick too low",
			env: map[string]string{
				"SCHEDULER_TICK_SECS": "2",
				"ENCRYPTION_KEY":      testHexKey(),
			},
			wantErr: "SCHEDULER_TICK_SECS must be at least 5",
		},
		{
			name: "invalid log level",
			env: map[string]string{
				"LOG_LEVEL":      "nope",
				"ENCRYPTION_KEY": testHexKey(),
			},
			wantErr: "not a valid log level",
		},
		{
			name:    "missing encryption key",
			env:     map[string]string{},
			wantErr: "ENCRYPTION_KEY is required",
		},
		{
			name: "wrong length encryption key",
			env: map[string]string{
				"ENCRYPTION_KEY": "tooshort",
			},
			wantErr: "64-character hex string or base64 encoding of 32 bytes",
		},
		{
			name: "valid hex encryption key",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.EncryptionKey) != 32 {
					t.Errorf("EncryptionKey length = %d, want 32",
						len(cfg.EncryptionKey))
				}
			},
		},
		{
			name: "valid base64 encryption key",
			env: map[string]string{
				"ENCRYPTION_KEY": testBase64Key(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.EncryptionKey) != 32 {
					t.Errorf("EncryptionKey length = %d, want 32",
						len(cfg.EncryptionKey))
				}
			},
		},
		{
			name: "auth both set",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
				"AUTH_USERNAME":  "admin",
				"AUTH_PASSWORD":  "secret",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if !cfg.AuthEnabled() {
					t.Error("AuthEnabled() = false, want true")
				}
				if cfg.AuthUsername != "admin" {
					t.Errorf("AuthUsername = %q, want %q",
						cfg.AuthUsername, "admin")
				}
				if cfg.AuthPassword != "secret" {
					t.Errorf("AuthPassword = %q, want %q",
						cfg.AuthPassword, "secret")
				}
			},
		},
		{
			name: "auth both empty",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.AuthEnabled() {
					t.Error("AuthEnabled() = true, want false")
				}
			},
		},
		{
			name: "auth username only",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
				"AUTH_USERNAME":  "admin",
			},
			wantErr: "AUTH_USERNAME and AUTH_PASSWORD must both be set or both be empty",
		},
		{
			name: "auth password only",
			env: map[string]string{
				"ENCRYPTION_KEY": testHexKey(),
				"AUTH_PASSWORD":  "secret",
			},
			wantErr: "AUTH_USERNAME and AUTH_PASSWORD must both be set or both be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range EnvKeys() {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil",
						tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q",
						err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadMultipleParseErrors(t *testing.T) {
	for _, key := range EnvKeys() {
		t.Setenv(key, "")
	}
	t.Setenv("PORT", "abc")
	t.Setenv("LOG_LEVEL", "nope")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "not a valid integer") {
		t.Errorf("error missing integer complaint: %s", msg)
	}
	if !strings.Contains(msg, "not a valid log level") {
		t.Errorf("error missing log level complaint: %s", msg)
	}
}
