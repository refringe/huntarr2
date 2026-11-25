package database

import (
	"testing"
	"time"
)

func TestFormatTimeSortsAsTextInTimeOrder(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	ordered := []time.Time{
		base,
		base.Add(500 * time.Millisecond),
		base.Add(510 * time.Millisecond),
		base.Add(time.Second),
	}

	for i := 1; i < len(ordered); i++ {
		earlier, later := FormatTime(ordered[i-1]), FormatTime(ordered[i])
		if earlier >= later {
			t.Errorf("FormatTime(%v) = %q, want it to sort before %q", ordered[i-1], earlier, later)
		}
	}
}

func TestFormatTimeConvertsToUTC(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("UTC+5", 5*60*60)
	local := time.Date(2026, 11, 24, 15, 0, 0, 0, zone)

	if got, want := FormatTime(local), "2026-11-24T10:00:00.000000000Z"; got != want {
		t.Errorf("FormatTime = %q, want %q", got, want)
	}
}

func TestParseTimeReadsBothWidths(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 11, 24, 10, 0, 0, 500_000_000, time.UTC)
	for _, stored := range []string{"2026-11-24T10:00:00.5Z", "2026-11-24T10:00:00.500000000Z", FormatTime(want)} {
		got, err := ParseTime(stored)
		if err != nil {
			t.Fatalf("ParseTime(%q): %v", stored, err)
		}
		if !got.Equal(want) {
			t.Errorf("ParseTime(%q) = %v, want %v", stored, got, want)
		}
	}
}
