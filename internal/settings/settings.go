// Package settings manages application settings stored as key/value pairs in SQLite. Settings may be global
// (instance_id IS NULL) or per-instance overrides.
package settings

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Setting keys used throughout the application; each corresponds to a typed field in Resolved.
const (
	KeyBatchSize         = "batch_size"
	KeyCooldownPeriod    = "cooldown_period"
	KeySearchWindowStart = "search_window_start"
	KeySearchWindowEnd   = "search_window_end"
	KeySearchInterval    = "search_interval"
	KeySearchLimit       = "search_limit"
	KeyEnabled           = "enabled"
	KeySearchMissing     = "search_missing"
)

var validKeys = map[string]struct{}{
	KeyBatchSize:         {},
	KeyCooldownPeriod:    {},
	KeySearchWindowStart: {},
	KeySearchWindowEnd:   {},
	KeySearchInterval:    {},
	KeySearchLimit:       {},
	KeyEnabled:           {},
	KeySearchMissing:     {},
}

// MaxCooldownPeriod is the longest cooldown period a setting may specify. Search cooldown records older than
// this can no longer match any configured cooldown period and are safe to prune.
const MaxCooldownPeriod = 90 * 24 * time.Hour

// MaxSearchInterval is the longest base search interval a setting may specify.
const MaxSearchInterval = 7 * 24 * time.Hour

// ErrUnknownKey is returned when an unrecognised setting key is provided.
var ErrUnknownKey = errors.New("unknown setting key")

// ErrInvalidValue is returned when a setting value cannot be parsed to the expected type for its key.
var ErrInvalidValue = errors.New("invalid setting value")

// SettingEntry is a key/value pair used when creating or updating settings in batch.
type SettingEntry struct {
	Key   string
	Value string
}

// Setting represents a single stored key/value pair. A nil InstanceID denotes a global setting; non-nil denotes a
// per-instance override.
type Setting struct {
	ID         uuid.UUID
	InstanceID *uuid.UUID
	Key        string
	Value      string
	UpdatedAt  time.Time
}

// Resolved holds the effective settings after merging defaults, global overrides, and per-instance overrides.
type Resolved struct {
	BatchSize         int           `json:"batchSize"`
	CooldownPeriod    time.Duration `json:"cooldownPeriod"`
	SearchWindowStart string        `json:"searchWindowStart"`
	SearchWindowEnd   string        `json:"searchWindowEnd"`
	SearchInterval    time.Duration `json:"searchInterval"`
	SearchLimit       int           `json:"searchLimit"`
	Enabled           bool          `json:"enabled"`
	SearchMissing     bool          `json:"searchMissing"`
}

// Defaults returns the compiled default settings used as the base layer before overrides are applied.
func Defaults() Resolved {
	return Resolved{
		BatchSize:         4,
		CooldownPeriod:    24 * time.Hour,
		SearchWindowStart: "",
		SearchWindowEnd:   "",
		SearchInterval:    30 * time.Minute,
		SearchLimit:       100,
		Enabled:           true,
		SearchMissing:     true,
	}
}

// ValidKey reports whether key is a recognised setting key.
func ValidKey(key string) bool {
	_, ok := validKeys[key]
	return ok
}

// ParseHHMM converts a "HH:MM" string to minutes since midnight, requiring exactly two digits for both components
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
