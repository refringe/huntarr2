package pages

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   int64
		want string
	}{
		{0, ""},
		{-5, ""},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{8_400_000_000, "7.8 GB"},
		{2_199_023_255_552, "2 TB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatBitrate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   int64
		want string
	}{
		{0, ""},
		{448_000, "448 kbps"},
		{1_000_000, "1 Mbps"},
		{12_150_000, "12.2 Mbps"},
	}
	for _, tt := range tests {
		if got := FormatBitrate(tt.in); got != tt.want {
			t.Errorf("FormatBitrate(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolutionLabel(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"1920x1080": "1080p",
		"3840x2160": "2160p",
		"":          "",
		"unknown":   "unknown",
		"axb":       "axb",
	}
	for in, want := range tests {
		if got := ResolutionLabel(in); got != want {
			t.Errorf("ResolutionLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAudioLabel(t *testing.T) {
	t.Parallel()
	if got := AudioLabel("TrueHD", 7.1); got != "TrueHD 7.1" {
		t.Errorf("got %q, want TrueHD 7.1", got)
	}
	if got := AudioLabel("AAC", 2); got != "AAC 2.0" {
		t.Errorf("got %q, want AAC 2.0", got)
	}
	if got := AudioLabel("", 5.1); got != "5.1" {
		t.Errorf("got %q, want 5.1", got)
	}
	if got := AudioLabel("", 0); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestRelativeTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{5 * time.Minute, "5 minutes ago"},
		{time.Hour, "1 hour ago"},
		{23 * time.Hour, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}
	for _, tt := range tests {
		if got := RelativeTime(now.Add(-tt.ago), now); got != tt.want {
			t.Errorf("RelativeTime(-%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestScoreLabel(t *testing.T) {
	t.Parallel()
	if got := ScoreLabel(nil); got != "?" {
		t.Errorf("ScoreLabel(nil) = %q, want ?", got)
	}
	score := -50
	if got := ScoreLabel(&score); got != "-50" {
		t.Errorf("ScoreLabel(-50) = %q, want -50", got)
	}
}

func TestHomeRendersLatestUpgrades(t *testing.T) {
	t.Parallel()
	score := 1100
	data := HomeData{
		HasArrInstances: true,
		LatestUpgrades: []HomeUpgrade{{
			InstanceName: "Radarr",
			AppType:      "Radarr",
			ItemLabel:    "Movie (2024)",
			ReleaseTitle: "Movie.2024.1080p-GRP",
			DetailURL:    "http://radarr.local/movie/movie-2024",
			PosterURL:    "/api/instances/abc/mediacover/42/poster-250.jpg",
			FromQuality:  "HDTV-720p",
			ToQuality:    "Bluray-1080p",
			FromSize:     2_000_000_000,
			ToSize:       8_000_000_000,
			ToScore:      &score,
			MediaTags:    []string{"1080p", "x265"},
			DetectedAt:   time.Now().Add(-2 * time.Hour),
		}},
	}

	var buf strings.Builder
	if err := Home(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		"Latest Upgrades",
		`href="http://radarr.local/movie/movie-2024"`,
		`src="/api/instances/abc/mediacover/42/poster-250.jpg"`,
		"HDTV-720p", "Bluray-1080p", "1.9 GB", "7.5 GB", ">?<", ">1100<", ">Quality<", ">Score<", ">Size<",
		"x265", "Movie.2024.1080p-GRP", "2 hours ago", "/logs?action=upgrade_detected",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered home page missing %q", want)
		}
	}

	buf.Reset()
	if err := Home(HomeData{HasArrInstances: true}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render empty: %v", err)
	}
	if !strings.Contains(buf.String(), "No quality upgrades detected yet") {
		t.Error("rendered home page missing the empty state")
	}

	buf.Reset()
	legacy := HomeData{HasArrInstances: true, LatestUpgrades: []HomeUpgrade{{ItemLabel: "Old", ToQuality: "Bluray-1080p"}}}
	if err := Home(legacy).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render legacy: %v", err)
	}
	if strings.Count(buf.String(), ">n/a<") != 2 {
		t.Errorf("legacy row should render n/a for score and size, got %d", strings.Count(buf.String(), ">n/a<"))
	}
}
