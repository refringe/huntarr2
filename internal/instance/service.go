package instance

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// defaultTimeoutMs is the timeout applied to new instances when the caller
// does not provide one.
const defaultTimeoutMs = 15000

// maxTimeoutMs is the upper bound for instance timeouts (5 minutes).
const maxTimeoutMs = 300000

// Validation field names and messages reused across validate.
const (
	fieldName      = "name"
	fieldAppType   = "app_type"
	fieldBaseURL   = "base_url"
	fieldAPIKey    = "api_key"
	fieldTimeoutMs = "timeout_ms"

	msgMustNotBeEmpty = "must not be empty"
	msgInvalidURL     = "must be a valid HTTP or HTTPS URL"
)

// Service provides business logic for managing instances.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns all instances.
func (s *Service) List(ctx context.Context) ([]Instance, error) {
	return s.repo.List(ctx)
}

// Get returns the instance with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Instance, error) {
	return s.repo.Get(ctx, id)
}

// Create validates and persists a new instance. Callers must set Name,
// AppType, BaseURL, and APIKey. TimeoutMs defaults to 15000 when zero.
func (s *Service) Create(ctx context.Context, inst *Instance) error {
	if inst.TimeoutMs == 0 {
		inst.TimeoutMs = defaultTimeoutMs
	}

	if err := validate(inst); err != nil {
		return err
	}

	return s.repo.Create(ctx, inst)
}

// Update applies changes to an existing instance and returns the updated
// value. The instance is fetched first so that AppType cannot be altered
// after creation. The returned Instance carries the database assigned
// updated_at timestamp.
func (s *Service) Update(ctx context.Context, id uuid.UUID, inst *Instance) (Instance, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return Instance{}, err
	}

	existing.Name = inst.Name
	existing.BaseURL = inst.BaseURL
	existing.APIKey = inst.APIKey
	if inst.TimeoutMs > 0 {
		existing.TimeoutMs = inst.TimeoutMs
	}

	if err := validate(&existing); err != nil {
		return Instance{}, err
	}

	if err := s.repo.Update(ctx, &existing); err != nil {
		return Instance{}, err
	}
	return existing, nil
}

// Delete removes the instance with the given ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// validate checks that all required fields on inst are present and
// sensible. Whitespace is trimmed from Name and APIKey before
// validation so that the stored values are clean.
func validate(inst *Instance) error {
	inst.Name = strings.TrimSpace(inst.Name)
	if inst.Name == "" {
		return &ValidationError{Field: fieldName, Message: msgMustNotBeEmpty}
	}

	if !inst.AppType.Valid() {
		return &ValidationError{
			Field:   fieldAppType,
			Message: fmt.Sprintf("%q is not a recognised application type", inst.AppType),
		}
	}

	u, err := url.Parse(inst.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return &ValidationError{
			Field:   fieldBaseURL,
			Message: msgInvalidURL,
		}
	}

	inst.APIKey = strings.TrimSpace(inst.APIKey)
	if inst.APIKey == "" {
		return &ValidationError{Field: fieldAPIKey, Message: msgMustNotBeEmpty}
	}

	if inst.TimeoutMs < 0 || inst.TimeoutMs > maxTimeoutMs {
		return &ValidationError{
			Field:   fieldTimeoutMs,
			Message: fmt.Sprintf("must be between 0 and %d", maxTimeoutMs),
		}
	}

	return nil
}
