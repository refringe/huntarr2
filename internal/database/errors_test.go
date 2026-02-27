package database_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/refringe/huntarr2/internal/database"
)

// fakeSQLiteError implements the sqliteError interface for unit testing.
type fakeSQLiteError struct {
	code int
	msg  string
}

func (e *fakeSQLiteError) Error() string { return e.msg }
func (e *fakeSQLiteError) Code() int     { return e.code }

// These constants mirror the unexported values in the database package.
const (
	uniqueViolation     = 2067
	foreignKeyViolation = 787
)

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique violation",
			err:  &fakeSQLiteError{code: uniqueViolation, msg: "UNIQUE constraint failed"},
			want: true,
		},
		{
			name: "wrapped unique violation",
			err:  fmt.Errorf("insert failed: %w", &fakeSQLiteError{code: uniqueViolation, msg: "UNIQUE"}),
			want: true,
		},
		{
			name: "foreign key violation",
			err:  &fakeSQLiteError{code: foreignKeyViolation, msg: "FOREIGN KEY"},
			want: false,
		},
		{
			name: "other sqlite error",
			err:  &fakeSQLiteError{code: 1, msg: "generic error"},
			want: false,
		},
		{
			name: "non-sqlite error",
			err:  errors.New("something else"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := database.IsUniqueViolation(tt.err); got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v",
					got, tt.want)
			}
		})
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "foreign key violation",
			err:  &fakeSQLiteError{code: foreignKeyViolation, msg: "FOREIGN KEY constraint failed"},
			want: true,
		},
		{
			name: "wrapped foreign key violation",
			err:  fmt.Errorf("insert failed: %w", &fakeSQLiteError{code: foreignKeyViolation, msg: "FK"}),
			want: true,
		},
		{
			name: "unique violation",
			err:  &fakeSQLiteError{code: uniqueViolation, msg: "UNIQUE"},
			want: false,
		},
		{
			name: "other sqlite error",
			err:  &fakeSQLiteError{code: 1, msg: "generic error"},
			want: false,
		},
		{
			name: "non-sqlite error",
			err:  errors.New("something else"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := database.IsForeignKeyViolation(tt.err); got != tt.want {
				t.Errorf("IsForeignKeyViolation() = %v, want %v",
					got, tt.want)
			}
		})
	}
}
