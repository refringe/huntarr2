package activity

import (
	"reflect"
	"testing"
)

func TestImportDetailsMapOmitsUnsetOptionalFields(t *testing.T) {
	t.Parallel()

	details, err := ImportDetails{ItemLabel: "Movie (2024)", Quality: "Bluray-1080p"}.Map()
	if err != nil {
		t.Fatalf("Map: %v", err)
	}

	want := map[string]any{
		"instanceName":    "",
		"instanceBaseURL": "",
		"itemLabel":       "Movie (2024)",
		"itemDetailPath":  "",
		"quality":         "Bluray-1080p",
	}
	if !reflect.DeepEqual(details, want) {
		t.Errorf("Map = %v, want %v", details, want)
	}
}

func TestImportDetailsRoundTrip(t *testing.T) {
	t.Parallel()

	score, previousScore := 1100, 0
	original := ImportDetails{
		InstanceName:              "Radarr",
		InstanceBaseURL:           "http://radarr.local",
		ItemLabel:                 "Movie (2024)",
		ItemDetailPath:            "/movie/movie-2024",
		Quality:                   "Bluray-1080p",
		ReleaseTitle:              "Movie.2024.1080p-GRP",
		ReleaseGroup:              "GRP",
		MediaCover:                "42/poster-250.jpg",
		Size:                      8_000_000_000,
		CustomFormatScore:         &score,
		PreviousQuality:           "HDTV-720p",
		PreviousSize:              2_000_000_000,
		PreviousCustomFormatScore: &previousScore,
		Resolution:                "1920x1080",
		VideoCodec:                "x265",
		VideoBitDepth:             10,
		VideoBitrate:              12_000_000,
		VideoDynamicRange:         "HDR10",
		AudioCodec:                "TrueHD",
		AudioChannels:             7.1,
		AudioBitrate:              4_500_000,
		AudioBitrateText:          "4.5 Mbps",
		AudioBits:                 "24bit",
		AudioSampleRate:           "48kHz",
		RunTime:                   "2:01:00",
	}

	details, err := original.Map()
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if _, ok := details["previousCustomFormatScore"]; !ok {
		t.Error("previousCustomFormatScore omitted, want a zero score kept")
	}

	got, err := ParseImportDetails(details)
	if err != nil {
		t.Fatalf("ParseImportDetails: %v", err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Errorf("round trip = %+v, want %+v", got, original)
	}
}

func TestParseImportDetailsRejectsMistypedValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseImportDetails(map[string]any{"size": "large"}); err == nil {
		t.Error("err = nil, want a decoding error for a string size")
	}
}
