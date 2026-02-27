// Package instance defines the domain types and errors for application
// instance management. An instance represents a configured connection to an
// *arr application (Sonarr, Radarr, Lidarr, or Whisparr).
package instance

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AppType identifies the type of *arr application an instance connects to.
type AppType string

// Application type constants for each supported *arr application.
const (
	AppTypeSonarr   AppType = "sonarr"
	AppTypeRadarr   AppType = "radarr"
	AppTypeLidarr   AppType = "lidarr"
	AppTypeWhisparr AppType = "whisparr"
)

// validAppTypes enumerates every recognised application type. Used by
// AppType.Valid for membership checks.
var validAppTypes = map[AppType]struct{}{
	AppTypeSonarr:   {},
	AppTypeRadarr:   {},
	AppTypeLidarr:   {},
	AppTypeWhisparr: {},
}

// Valid reports whether t is a recognised application type.
func (t AppType) Valid() bool {
	_, ok := validAppTypes[t]
	return ok
}

// Instance represents a configured connection to an *arr application.
type Instance struct {
	ID        uuid.UUID
	Name      string
	AppType   AppType
	BaseURL   string
	APIKey    string
	TimeoutMs int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrNotFound is returned when a requested instance does not exist.
var ErrNotFound = errors.New("instance not found")

// ErrValidation is the sentinel wrapped by ValidationError.
var ErrValidation = errors.New("validation error")

// ValidationError describes a single field that failed validation.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}
