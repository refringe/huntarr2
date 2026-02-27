package main

import (
	"strings"
	"testing"

	"github.com/refringe/huntarr2/internal/config"
)

func TestRunFailsOnUnreachableDatabasePath(t *testing.T) {
	for _, key := range config.EnvKeys() {
		t.Setenv(key, "")
	}
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("DATABASE_PATH", "/nonexistent/dir/test.db")

	err := run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "opening database") {
		t.Errorf("error %q does not mention opening database", err.Error())
	}
}

func TestRunRequiresEncryptionKey(t *testing.T) {
	for _, key := range config.EnvKeys() {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_PATH", "/tmp/test.db")

	err := run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Errorf("error %q does not mention ENCRYPTION_KEY", err.Error())
	}
}
