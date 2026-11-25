package database

import "time"

// timeLayout is RFC 3339 in UTC with a fixed nine fractional digits.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// FormatTime renders t in UTC for storage in, or comparison against, a TEXT timestamp column.
func FormatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

// ParseTime reads a stored TEXT timestamp, including values written with fewer fractional digits.
func ParseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
