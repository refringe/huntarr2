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

// Resolve merges compiled defaults, global overrides, and per-instance overrides into a single Resolved struct;
// precedence is per-instance > global > defaults.
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

// ResolveGlobal merges compiled defaults with global overrides only (no per-instance layer).
func (s *Service) ResolveGlobal(ctx context.Context) (Resolved, error) {
	resolved := Defaults()

	global, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return Resolved{}, fmt.Errorf("loading global settings: %w", err)
	}
	applyOverrides(&resolved, global)

	return resolved, nil
}

// Set validates and persists a setting; a nil instanceID sets a global value, non-nil a per-instance override.
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

// SetBatch validates every entry before atomically persisting them all; a nil instanceID sets global values,
// non-nil per-instance overrides.
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

// Remove deletes a setting override; a nil instanceID removes the global value, non-nil the per-instance override.
func (s *Service) Remove(ctx context.Context, instanceID *uuid.UUID, key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
	}
	return s.repo.Delete(ctx, instanceID, key)
}

// RemoveBatch validates every key before atomically deleting the matching setting overrides.
func (s *Service) RemoveBatch(ctx context.Context, instanceID *uuid.UUID, keys []string) error {
	for _, key := range keys {
		if !ValidKey(key) {
			return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
		}
	}
	return s.repo.DeleteBatch(ctx, instanceID, keys)
}

// applyOverrides applies stored settings to the resolved struct, logging and skipping values that fail to parse.
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
			if v, err := time.ParseDuration(s.Value); err != nil || v <= 0 {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			} else if v > MaxCooldownPeriod {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("clamping stored cooldown period to the maximum")
				r.CooldownPeriod = MaxCooldownPeriod
			} else {
				r.CooldownPeriod = v
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
		case KeySearchMissing:
			if v, err := strconv.ParseBool(s.Value); err == nil {
				r.SearchMissing = v
			} else {
				log.Warn().Str("key", s.Key).Str("value", s.Value).
					Msg("ignoring unparsable setting")
			}
		}
	}
}

// ValidateEntry checks that key is recognised and that value is parsable and within acceptable bounds for that key.
func ValidateEntry(key, value string) error {
	if !ValidKey(key) {
		return fmt.Errorf("key %q: %w", key, ErrUnknownKey)
	}
	return validateValue(key, value)
}

// validateValue checks that the raw string is parsable and within acceptable bounds for the given key.
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
	case KeyCooldownPeriod:
		v, parseErr := time.ParseDuration(value)
		if parseErr != nil {
			err = parseErr
		} else if v <= 0 || v > MaxCooldownPeriod {
			err = fmt.Errorf("must be positive and at most %.0fh", MaxCooldownPeriod.Hours())
		}
	case KeySearchInterval:
		_, err = time.ParseDuration(value)
	case KeyEnabled, KeySearchMissing:
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

func validateHHMM(v string) error {
	_, err := ParseHHMM(v)
	return err
}
