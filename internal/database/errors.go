package database

import (
	"errors"

	sqlite3 "modernc.org/sqlite/lib"
)

// SQLite extended result codes for constraint violations.
const (
	uniqueViolation     = sqlite3.SQLITE_CONSTRAINT_UNIQUE
	foreignKeyViolation = sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
)

// sqliteError mirrors the error interface exposed by modernc.org/sqlite.
// Using a local interface avoids importing the top-level sqlite package
// solely for the error type.
type sqliteError interface {
	error
	Code() int
}

// IsUniqueViolation reports whether err is a SQLite unique constraint
// violation.
func IsUniqueViolation(err error) bool {
	if se, ok := errors.AsType[sqliteError](err); ok {
		return se.Code() == uniqueViolation
	}
	return false
}

// IsForeignKeyViolation reports whether err is a SQLite foreign key
// constraint violation.
func IsForeignKeyViolation(err error) bool {
	if se, ok := errors.AsType[sqliteError](err); ok {
		return se.Code() == foreignKeyViolation
	}
	return false
}
