package arr

import (
	"testing"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

func TestNewApp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType instance.AppType
		wantErr bool
	}{
		{"sonarr succeeds", instance.AppTypeSonarr, false},
		{"radarr succeeds", instance.AppTypeRadarr, false},
		{"lidarr succeeds", instance.AppTypeLidarr, false},
		{"whisparr succeeds", instance.AppTypeWhisparr, false},
		{"unknown type errors", instance.AppType("bogus"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, err := NewApp(tc.appType, "http://localhost:8989", "key", 5*time.Second)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.appType)
				}
				if app != nil {
					t.Errorf("expected nil app on error for %q", tc.appType)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tc.appType, err)
				}
				if app == nil {
					t.Errorf("expected non-nil app for %q", tc.appType)
				}
			}
		})
	}
}
