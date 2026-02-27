// Package settings manages application settings stored as key/value pairs in
// SQLite. Settings may be global (instance_id IS NULL) or per-instance
// overrides. The Resolve method merges compiled defaults, global overrides,
// and per-instance overrides into a single Resolved struct.
package settings

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Setting keys used throughout the application. Each key corresponds to a
// typed field in Resolved.
const (
	KeyBatchSize         = "batch_size"
	KeyCooldownPeriod    = "cooldown_period"
	KeySearchWindowStart = "search_window_start"
	KeySearchWindowEnd   = "search_window_end"
	KeySearchInterval    = "search_interval"
	KeySearchLimit       = "search_limit"
	KeyEnabled           = "enabled"
)

// validKeys enumerates every recognised setting key. Used by ValidKey to
// perform membership checks.
var validKeys = map[string]struct{}{
	KeyBatchSize:         {},
	KeyCooldownPeriod:    {},
	KeySearchWindowStart: {},
	KeySearchWindowEnd:   {},
	KeySearchInterval:    {},
	KeySearchLimit:       {},
	KeyEnabled:           {},
}

// ErrUnknownKey is returned when an unrecognised setting key is provided.
var ErrUnknownKey = errors.New("unknown setting key")

// ErrInvalidValue is returned when a setting value cannot be parsed to the
// expected type for its key.
var ErrInvalidValue = errors.New("invalid setting value")

// SettingEntry is a key/value pair used when creating or updating settings
// in batch. It is the public input type for Service.SetBatch.
type SettingEntry struct {
	Key   string
	Value string
}

// Setting represents a single stored key/value pair. A nil InstanceID
// denotes a global setting; non-nil denotes a per-instance override.
type Setting struct {
	ID         uuid.UUID
	InstanceID *uuid.UUID
	Key        string
	Value      string
	UpdatedAt  time.Time
}

// Resolved holds the effective settings for a given context after merging
// compiled defaults, global overrides, and per-instance overrides.
type Resolved struct {
	BatchSize         int           `json:"batchSize"`
	CooldownPeriod    time.Duration `json:"cooldownPeriod"`
	SearchWindowStart string        `json:"searchWindowStart"`
	SearchWindowEnd   string        `json:"searchWindowEnd"`
	SearchInterval    time.Duration `json:"searchInterval"`
	SearchLimit       int           `json:"searchLimit"`
	Enabled           bool          `json:"enabled"`
}

// Defaults returns the compiled default settings used as the base layer
// before global and per-instance overrides are applied.
func Defaults() Resolved {
	return Resolved{
		BatchSize:         4,
		CooldownPeriod:    24 * time.Hour,
		SearchWindowStart: "",
		SearchWindowEnd:   "",
		SearchInterval:    30 * time.Minute,
		SearchLimit:       100,
		Enabled:           true,
	}
}

// ValidKey reports whether key is a recognised setting key.
func ValidKey(key string) bool {
	_, ok := validKeys[key]
	return ok
}

// ParseHHMM converts a "HH:MM" string to minutes since midnight. It
// requires exactly two digits for both the hour and minute components
// (e.g. "09:30" is valid, "9:30" is not).
func ParseHHMM(v string) (int, error) {
	if len(v) != 5 || v[2] != ':' {
		return 0, fmt.Errorf("expected HH:MM format, got %q", v)
	}
	h, err := strconv.Atoi(v[:2])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in %q", v)
	}
	m, err := strconv.Atoi(v[3:])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", v)
	}
	return h*60 + m, nil
}
