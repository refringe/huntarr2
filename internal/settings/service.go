package settings

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Service provides operations for managing and resolving settings.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Resolve merges compiled defaults, global overrides, and per-instance
// overrides into a single Resolved struct. The precedence order is:
// per-instance > global > defaults.
func (s *Service) Resolve(ctx context.Context, instanceID uuid.UUID) (Resolved, error) {
	resolved := Defaults()

	global, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return Resolved{}, fmt.Errorf("loading global settings: %w", err)
	}
	applyOverrides(&resolved, global)

	perInst, err := s.repo.ListByInstance(ctx, instanceID)
	if err != nil {
		return Resolved{}, fmt.Errorf("loading instance settings: %w", err)
	}
	applyOverrides(&resolved, perInst)

	return resolved, nil
}

// ResolveGlobal merges compiled defaults with global overrides only (no
// per-instance layer).
func (s *Service) ResolveGlobal(ctx context.Context) (Resolved, error) {
	resolved := Defaults()

	global, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return Resolved{}, fmt.Errorf("loading global settings: %w", err)
	}
	applyOverrides(&resolved, global)

	return resolved, nil
}

// Set validates and persists a setting. A nil instanceID sets a global
// value; non-nil sets a per-instance override.
func (s *Service) Set(ctx context.Context, instanceID *uuid.UUID, key, value string) error {
	if !ValidKey(key) {
		return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
	}
	if err := validateValue(key, value); err != nil {
		return err
	}
	return s.repo.Upsert(ctx, &Setting{
		InstanceID: instanceID,
		Key:        key,
		Value:      value,
	})
}

// SetBatch validates and atomically persists multiple settings. All entries
// are validated before any are written; if validation fails for any entry,
// no changes are committed. A nil instanceID sets global values; non-nil
// sets per-instance overrides.
func (s *Service) SetBatch(ctx context.Context, instanceID *uuid.UUID, entries []SettingEntry) error {
	for _, e := range entries {
		if !ValidKey(e.Key) {
			return fmt.Errorf("key %q: %w", e.Key, ErrUnknownKey)
		}
		if err := validateValue(e.Key, e.Value); err != nil {
			return err
		}
	}

	settings := make([]Setting, len(entries))
	for i, e := range entries {
		settings[i] = Setting{
			InstanceID: instanceID,
			Key:        e.Key,
			Value:      e.Value,
		}
	}
	return s.repo.UpsertBatch(ctx, settings)
}

// Remove deletes a setting override. A nil instanceID removes the
// global value; non-nil removes the per-instance override.
func (s *Service) Remove(ctx context.Context, instanceID *uuid.UUID, key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
	}
	return s.repo.Delete(ctx, instanceID, key)
}

// RemoveBatch validates and atomically deletes multiple setting
// overrides. All keys are validated before any are removed; if
// validation fails for any key, no changes are committed.
func (s *Service) RemoveBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error {
	for _, key := range keys {
		if !ValidKey(key) {
			return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
		}
	}
	return s.repo.DeleteBatch(ctx, instanceID, keys)
}

// applyOverrides applies a slice of stored settings to the resolved struct.
// Values that fail to parse are logged as warnings and skipped so that one
// corrupt row does not prevent the rest of the settings from loading.
func applyOverrides(r *Resolved, settings []Setting) {
	for _, s := range settings {
		switch s.Key {
		case KeyBatchSize:
			if v, err := strconv.Atoi(s.Value); err == nil {
				r.BatchSize = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		case KeyCooldownPeriod:
			if v, err := time.ParseDuration(s.Value); err == nil {
				r.CooldownPeriod = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		case KeySearchWindowStart:
			r.SearchWindowStart = s.Value
		case KeySearchWindowEnd:
			r.SearchWindowEnd = s.Value
		case KeySearchInterval:
			if v, err := time.ParseDuration(s.Value); err == nil {
				r.SearchInterval = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		case KeySearchLimit:
			if v, err := strconv.Atoi(s.Value); err == nil {
				r.SearchLimit = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		case KeyEnabled:
			if v, err := strconv.ParseBool(s.Value); err == nil {
				r.Enabled = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		}
	}
}

// ValidateEntry checks that key is recognised and that value is parsable
// and within acceptable bounds for that key. This is exposed so that
// callers (such as API handlers) can validate a batch of entries before
// persisting any of them.
func ValidateEntry(key, value string) error {
	if !ValidKey(key) {
		return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
	}
	return validateValue(key, value)
}

// validateValue checks that the raw string is parsable and within acceptable
// bounds for the given key.
func validateValue(key, value string) error {
	var err error
	switch key {
	case KeyBatchSize, KeySearchLimit:
		v, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			err = parseErr
		} else if v < 1 {
			err = fmt.Errorf("must be a positive integer")
		}
	case KeyCooldownPeriod, KeySearchInterval:
		_, err = time.ParseDuration(value)
	case KeyEnabled:
		_, err = strconv.ParseBool(value)
	case KeySearchWindowStart, KeySearchWindowEnd:
		if value != "" {
			err = validateHHMM(value)
		}
	}
	if err != nil {
		return fmt.Errorf("key %q value %q: %w", key, value, ErrInvalidValue)
	}
	return nil
}

// validateHHMM checks that v is in "HH:MM" format with valid hour/minute
// ranges.
func validateHHMM(v string) error {
	_, err := ParseHHMM(v)
	return err
}
