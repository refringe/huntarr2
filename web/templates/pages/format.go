package pages

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// FormatBytes renders a byte count with a binary-scaled unit (e.g. "8.4 GB"), or an empty string for zero.
func FormatBytes(n int64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(n)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", n, units[unit])
	}
	return fmt.Sprintf("%s %s", trimDecimal(value), units[unit])
}

// FormatBitrate renders a bits-per-second value as kbps or Mbps (e.g. "12.1 Mbps"), or an empty string for zero.
func FormatBitrate(bps int64) string {
	switch {
	case bps <= 0:
		return ""
	case bps >= 1_000_000:
		return trimDecimal(float64(bps)/1_000_000) + " Mbps"
	default:
		return strconv.FormatInt(int64(math.Round(float64(bps)/1_000)), 10) + " kbps"
	}
}

// trimDecimal formats a value with one decimal place, dropping the decimal when it is zero.
func trimDecimal(value float64) string {
	s := strconv.FormatFloat(value, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

// ResolutionLabel converts a "WIDTHxHEIGHT" resolution into its height label (e.g. "1080p"), else returns the input.
func ResolutionLabel(resolution string) string {
	_, height, ok := strings.Cut(resolution, "x")
	if !ok {
		return resolution
	}
	if _, err := strconv.Atoi(height); err != nil {
		return resolution
	}
	return height + "p"
}

// AudioLabel joins an audio codec with its channel layout (e.g. "TrueHD 7.1"), omitting whichever part is missing.
func AudioLabel(codec string, channels float64) string {
	var parts []string
	if codec != "" {
		parts = append(parts, codec)
	}
	if channels > 0 {
		parts = append(parts, strconv.FormatFloat(channels, 'f', 1, 64))
	}
	return strings.Join(parts, " ")
}

// RelativeTime renders how long before now t occurred in the coarsest whole unit (e.g. "3 hours ago").
func RelativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour") + " ago"
	default:
		return plural(int(d/(24*time.Hour)), "day") + " ago"
	}
}

// plural renders a count with its unit, adding an "s" when the count is not one.
func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// ScoreLabel renders an optional custom format score, or "?" when unknown.
func ScoreLabel(score *int) string {
	if score == nil {
		return "?"
	}
	return strconv.Itoa(*score)
}

// orUnknown returns "Unknown" in place of an empty value.
func orUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
}
