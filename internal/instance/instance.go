// Package instance defines the domain types and errors for application
// instance management. An instance represents a configured connection to an
// *arr application (Sonarr, Radarr, Lidarr, or Whisparr v2/v3).
package instance

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// AppType identifies the type of *arr application an instance connects to.
type AppType string

// Application type constants for each supported *arr application. The two
// Whisparr generations are separate types: v2 is episode-based (a Sonarr
// fork) and v3 "Eros" is movie-based (a Radarr fork).
const (
	AppTypeSonarr     AppType = "sonarr"
	AppTypeRadarr     AppType = "radarr"
	AppTypeLidarr     AppType = "lidarr"
	AppTypeWhisparrV2 AppType = "whisparr-v2"
	AppTypeWhisparrV3 AppType = "whisparr-v3"
)

// allAppTypes lists every recognised application type in display order.
// validAppTypes and appTypeLabels are keyed on the same set; the guard
// test in service_test.go keeps them in sync.
var allAppTypes = []AppType{
	AppTypeSonarr,
	AppTypeRadarr,
	AppTypeLidarr,
	AppTypeWhisparrV2,
	AppTypeWhisparrV3,
}

// appTypeLabels maps each application type to its human-readable name.
var appTypeLabels = map[AppType]string{
	AppTypeSonarr:     "Sonarr",
	AppTypeRadarr:     "Radarr",
	AppTypeLidarr:     "Lidarr",
	AppTypeWhisparrV2: "Whisparr V2",
	AppTypeWhisparrV3: "Whisparr V3",
}

// validAppTypes enumerates every recognised application type. Used by
// AppType.Valid for membership checks.
var validAppTypes = func() map[AppType]struct{} {
	m := make(map[AppType]struct{}, len(allAppTypes))
	for _, t := range allAppTypes {
		m[t] = struct{}{}
	}
	return m
}()

// AllAppTypes returns every recognised application type in display order.
func AllAppTypes() []AppType {
	return slices.Clone(allAppTypes)
}

// Valid reports whether t is a recognised application type.
func (t AppType) Valid() bool {
	_, ok := validAppTypes[t]
	return ok
}

// Label returns the human-readable name for t, falling back to the raw
// value for unrecognised types.
func (t AppType) Label() string {
	if label, ok := appTypeLabels[t]; ok {
		return label
	}
	return string(t)
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
