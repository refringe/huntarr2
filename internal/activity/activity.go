// Package activity provides a structured activity log stored in SQLite. The
// scheduler writes entries as it runs; the UI reads them for display.
package activity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Level classifies the severity of an activity log entry.
type Level string

// Log level constants matching the database CHECK constraint.
const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Action classifies what the scheduler was doing when it logged the entry.
type Action string

// Action constants matching the database CHECK constraint.
const (
	ActionSearchCycle      Action = "search_cycle"
	ActionSearchSkip       Action = "search_skip"
	ActionRateLimit        Action = "rate_limit"
	ActionHealthCheck      Action = "health_check"
	ActionUpgradeDetected  Action = "upgrade_detected"
	ActionDownloadDetected Action = "download_detected"
)

// ErrInvalidLevel is returned when an unrecognised level is provided.
var ErrInvalidLevel = errors.New("invalid activity level")

// ErrInvalidAction is returned when an unrecognised action is provided.
var ErrInvalidAction = errors.New("invalid activity action")

// validLevels enumerates every recognised level.
var validLevels = map[Level]struct{}{
	LevelDebug: {},
	LevelInfo:  {},
	LevelWarn:  {},
	LevelError: {},
}

// validActions enumerates every recognised action.
var validActions = map[Action]struct{}{
	ActionSearchCycle:      {},
	ActionSearchSkip:       {},
	ActionRateLimit:        {},
	ActionHealthCheck:      {},
	ActionUpgradeDetected:  {},
	ActionDownloadDetected: {},
}

// ValidLevel reports whether level is a recognised activity log level.
func ValidLevel(level Level) bool {
	_, ok := validLevels[level]
	return ok
}

// ValidAction reports whether action is a recognised activity log action.
func ValidAction(action Action) bool {
	_, ok := validActions[action]
	return ok
}

// Entry represents a single activity log record.
type Entry struct {
	ID         uuid.UUID      `json:"id"`
	InstanceID *uuid.UUID     `json:"instanceId,omitempty"`
	Level      Level          `json:"level"`
	Action     Action         `json:"action"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details"`
	CreatedAt  time.Time      `json:"createdAt"`
}
