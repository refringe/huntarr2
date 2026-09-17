package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
)

// parser accumulates parse errors from environment variable reads.
type parser struct {
	errs []error
}

func (p *parser) intVal(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: %q is not a valid integer", key, raw))
		return fallback
	}
	return v
}

func (p *parser) logLevel(key string, fallback zerolog.Level) zerolog.Level {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	lvl, err := zerolog.ParseLevel(raw)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: %q is not a valid log level", key, raw))
		return fallback
	}
	return lvl
}

// encryptionKey reads a 32-byte key given as 64 hex characters or base64, recording an error for anything else.
func (p *parser) encryptionKey(key string) []byte {
	raw := os.Getenv(key)
	if raw == "" {
		p.errs = append(p.errs, fmt.Errorf("%s is required", key))
		return nil
	}

	if len(raw) == 64 {
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
			return decoded
		}
	}

	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded
	}

	p.errs = append(p.errs, fmt.Errorf(
		"%s: value must be a 64-character hex string or base64 encoding of 32 bytes",
		key,
	))
	return nil
}
