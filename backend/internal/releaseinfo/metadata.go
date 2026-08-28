package releaseinfo

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// Metadata is the canonical build and release identity embedded in every binary.
type Metadata struct {
	SchemaVersion         int    `json:"schema_version"`
	ReworkVersion         string `json:"rework_version"`
	UpstreamBaseline      string `json:"upstream_baseline"`
	UpstreamBaselineSHA   string `json:"upstream_baseline_sha"`
	UpstreamRepository    string `json:"upstream_repository"`
	ReworkRepository      string `json:"rework_repository"`
	ArtifactRepository    string `json:"artifact_repository"`
	UpdateChannel         string `json:"update_channel"`
	DefaultPolicy         string `json:"default_policy"`
	MinimumUpdaterVersion string `json:"minimum_updater_version"`
	MigrationMin          int    `json:"migration_min"`
	MigrationMax          int    `json:"migration_max"`
}

//go:embed metadata.json
var rawMetadata []byte

var embedded = mustParse(rawMetadata)

// Current returns a copy of the embedded release metadata.
func Current() Metadata {
	return embedded
}

// JSON returns a copy of the canonical metadata document.
func JSON() []byte {
	return append([]byte(nil), rawMetadata...)
}

func mustParse(data []byte) Metadata {
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		panic(fmt.Sprintf("invalid embedded release metadata: %v", err))
	}
	if metadata.SchemaVersion != 1 || metadata.ReworkVersion == "" || metadata.UpstreamBaseline == "" ||
		metadata.UpstreamBaselineSHA == "" || metadata.UpstreamRepository == "" ||
		metadata.ReworkRepository == "" || metadata.ArtifactRepository == "" ||
		metadata.UpdateChannel == "" || metadata.DefaultPolicy != "manual" || metadata.MinimumUpdaterVersion == "" ||
		metadata.MigrationMin < 0 || metadata.MigrationMax < metadata.MigrationMin {
		panic("invalid embedded release metadata values")
	}
	return metadata
}
