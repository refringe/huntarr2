package activity

import (
	"encoding/json/v2"
	"fmt"
)

// ImportDetails is the details payload of an upgrade_detected or download_detected entry.
type ImportDetails struct {
	InstanceName    string `json:"instanceName"`
	InstanceBaseURL string `json:"instanceBaseURL"`
	ItemLabel       string `json:"itemLabel"`
	ItemDetailPath  string `json:"itemDetailPath"`
	Quality         string `json:"quality"`

	ReleaseTitle      string `json:"releaseTitle,omitzero"`
	ReleaseGroup      string `json:"releaseGroup,omitzero"`
	MediaCover        string `json:"mediaCover,omitzero"`
	Size              int64  `json:"size,omitzero"`
	CustomFormatScore *int   `json:"customFormatScore,omitzero"`

	PreviousQuality           string `json:"previousQuality,omitzero"`
	PreviousSize              int64  `json:"previousSize,omitzero"`
	PreviousCustomFormatScore *int   `json:"previousCustomFormatScore,omitzero"`

	Resolution        string  `json:"resolution,omitzero"`
	VideoCodec        string  `json:"videoCodec,omitzero"`
	VideoBitDepth     int     `json:"videoBitDepth,omitzero"`
	VideoBitrate      int64   `json:"videoBitrate,omitzero"`
	VideoDynamicRange string  `json:"videoDynamicRange,omitzero"`
	AudioCodec        string  `json:"audioCodec,omitzero"`
	AudioChannels     float64 `json:"audioChannels,omitzero"`
	AudioBitrate      int64   `json:"audioBitrate,omitzero"`
	AudioBitrateText  string  `json:"audioBitrateText,omitzero"`
	AudioBits         string  `json:"audioBits,omitzero"`
	AudioSampleRate   string  `json:"audioSampleRate,omitzero"`
	RunTime           string  `json:"runTime,omitzero"`
}

// Map returns the details in the map form carried by Entry.Details.
func (d ImportDetails) Map() (map[string]any, error) {
	encoded, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("encoding import details: %w", err)
	}
	var details map[string]any
	if err := json.Unmarshal(encoded, &details); err != nil {
		return nil, fmt.Errorf("decoding import details: %w", err)
	}
	return details, nil
}

// ParseImportDetails reads an import entry's details map back into its typed form.
func ParseImportDetails(details map[string]any) (ImportDetails, error) {
	encoded, err := json.Marshal(details)
	if err != nil {
		return ImportDetails{}, fmt.Errorf("encoding entry details: %w", err)
	}
	var d ImportDetails
	if err := json.Unmarshal(encoded, &d); err != nil {
		return ImportDetails{}, fmt.Errorf("decoding import details: %w", err)
	}
	return d, nil
}
