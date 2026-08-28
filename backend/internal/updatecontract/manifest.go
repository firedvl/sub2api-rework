package updatecontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	SchemaVersion    = 1
	MaxManifestBytes = 64 * 1024
	MaxNoteBytes     = 8 * 1024
)

var (
	reworkVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-rework\.[0-9]+$`)
	upstreamPattern      = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	shaPattern           = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern        = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Compatibility string

const (
	CompatibilityPending  Compatibility = "pending"
	CompatibilityApproved Compatibility = "approved"
	CompatibilityBlocked  Compatibility = "blocked"
)

// Manifest is the complete, immutable contract for one approved rework image.
type Manifest struct {
	SchemaVersion         int           `json:"schema_version"`
	ReworkVersion         string        `json:"rework_version"`
	UpstreamVersion       string        `json:"upstream_version"`
	GitSHA                string        `json:"git_sha"`
	Image                 string        `json:"image"`
	ImageDigest           string        `json:"image_digest"`
	MigrationMin          int           `json:"migration_min"`
	MigrationMax          int           `json:"migration_max"`
	ReleaseDate           string        `json:"release_date"`
	Compatibility         Compatibility `json:"compatibility"`
	MinimumUpdaterVersion string        `json:"minimum_updater_version"`
	ReleaseNotes          ReleaseNotes  `json:"release_notes"`
}

type ReleaseNotes struct {
	Upstream      string `json:"upstream"`
	Rework        string `json:"rework"`
	Compatibility string `json:"compatibility"`
	Migrations    string `json:"migrations"`
	Rollback      string `json:"rollback"`
}

// Parse rejects unknown fields, trailing values, oversized input, and invalid policy data.
func Parse(data []byte, trustedRepository string) (*Manifest, error) {
	if len(data) == 0 || len(data) > MaxManifestBytes {
		return nil, fmt.Errorf("manifest size must be between 1 and %d bytes", MaxManifestBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := manifest.Validate(trustedRepository); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("manifest contains trailing JSON data")
		}
		return fmt.Errorf("decode trailing manifest data: %w", err)
	}
	return nil
}

func (m Manifest) Validate(trustedRepository string) error {
	trustedRepository = strings.TrimSuffix(strings.TrimSpace(trustedRepository), "/")
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported manifest schema_version %d", m.SchemaVersion)
	}
	if !reworkVersionPattern.MatchString(m.ReworkVersion) || !semver.IsValid("v"+m.ReworkVersion) {
		return errors.New("invalid rework_version")
	}
	if !upstreamPattern.MatchString(m.UpstreamVersion) || !semver.IsValid(m.UpstreamVersion) {
		return errors.New("invalid upstream_version")
	}
	if !shaPattern.MatchString(m.GitSHA) {
		return errors.New("invalid git_sha")
	}
	if trustedRepository == "" || m.Image != trustedRepository+":"+m.ReworkVersion {
		return errors.New("image does not match the trusted repository and release version")
	}
	if !digestPattern.MatchString(m.ImageDigest) {
		return errors.New("invalid image_digest")
	}
	if m.MigrationMin < 0 || m.MigrationMax < m.MigrationMin {
		return errors.New("invalid migration range")
	}
	if _, err := time.Parse(time.RFC3339, m.ReleaseDate); err != nil {
		return errors.New("invalid release_date")
	}
	switch m.Compatibility {
	case CompatibilityPending, CompatibilityApproved, CompatibilityBlocked:
	default:
		return errors.New("invalid compatibility state")
	}
	if !plainVersionValid(m.MinimumUpdaterVersion) {
		return errors.New("invalid minimum_updater_version")
	}
	for name, value := range map[string]string{
		"upstream": m.ReleaseNotes.Upstream, "rework": m.ReleaseNotes.Rework,
		"compatibility": m.ReleaseNotes.Compatibility, "migrations": m.ReleaseNotes.Migrations,
		"rollback": m.ReleaseNotes.Rollback,
	} {
		if len(value) > MaxNoteBytes {
			return fmt.Errorf("release_notes.%s exceeds %d bytes", name, MaxNoteBytes)
		}
	}
	return nil
}

func plainVersionValid(version string) bool {
	if version == "" || strings.HasPrefix(version, "v") {
		return false
	}
	return semver.IsValid("v" + version)
}

func (m Manifest) ImmutableImage() string {
	return m.Image + "@" + m.ImageDigest
}

func IsReworkVersion(value string) bool {
	return reworkVersionPattern.MatchString(value) && semver.IsValid("v"+value)
}

func CompareRework(a, b string) int {
	return semver.Compare("v"+a, "v"+b)
}
