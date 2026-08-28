//go:build unit

package updatecontract

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/releaseinfo"
	"github.com/stretchr/testify/require"
)

func TestReleaseManifestFromEnvironment(t *testing.T) {
	path := os.Getenv("SUB2API_RELEASE_MANIFEST")
	if path == "" {
		t.Skip("release manifest path is not configured")
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	metadata := releaseinfo.Current()
	manifest, err := Parse(data, metadata.ArtifactRepository)
	require.NoError(t, err)
	require.Equal(t, metadata.ReworkVersion, manifest.ReworkVersion)
	require.Equal(t, metadata.UpstreamBaseline, manifest.UpstreamVersion)
	require.Equal(t, metadata.MinimumUpdaterVersion, manifest.MinimumUpdaterVersion)
}

const trustedRepository = "ghcr.io/firedvl/sub2api-rework"

func validManifest() Manifest {
	return Manifest{
		SchemaVersion:         1,
		ReworkVersion:         "0.1.184-rework.1",
		UpstreamVersion:       "v0.1.184",
		GitSHA:                strings.Repeat("a", 40),
		Image:                 trustedRepository + ":0.1.184-rework.1",
		ImageDigest:           "sha256:" + strings.Repeat("b", 64),
		MigrationMin:          232,
		MigrationMax:          233,
		ReleaseDate:           "2026-08-28T12:00:00Z",
		Compatibility:         CompatibilityApproved,
		MinimumUpdaterVersion: "1.0.0",
		ReleaseNotes: ReleaseNotes{
			Upstream: "Upstream changes", Rework: "Rework changes",
			Compatibility: "Qualified", Migrations: "Adds migration 233",
			Rollback: "Manual rollback blocks after schema advancement",
		},
	}
}

func encodeManifest(t *testing.T, manifest Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	return data
}

func TestParseAcceptsApprovedTrustedManifest(t *testing.T) {
	manifest, err := Parse(encodeManifest(t, validManifest()), trustedRepository)
	require.NoError(t, err)
	require.Equal(t, trustedRepository+":0.1.184-rework.1@"+manifest.ImageDigest, manifest.ImmutableImage())
}

func TestParseRejectsUnknownFieldAndTrailingData(t *testing.T) {
	data := encodeManifest(t, validManifest())
	data = append(data[:len(data)-1], []byte(`,"command":"sh"}`)...)
	_, err := Parse(data, trustedRepository)
	require.ErrorContains(t, err, "unknown field")

	_, err = Parse(append(encodeManifest(t, validManifest()), []byte(` {}`)...), trustedRepository)
	require.ErrorContains(t, err, "trailing")
}

func TestParseRejectsUntrustedImageAndMalformedDigest(t *testing.T) {
	manifest := validManifest()
	manifest.Image = "ghcr.io/attacker/sub2api:0.1.184-rework.1"
	_, err := Parse(encodeManifest(t, manifest), trustedRepository)
	require.ErrorContains(t, err, "trusted repository")

	manifest = validManifest()
	manifest.ImageDigest = "sha256:abcd"
	_, err = Parse(encodeManifest(t, manifest), trustedRepository)
	require.ErrorContains(t, err, "image_digest")
}

func TestParseRejectsInstallabilityPolicyErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"version", func(m *Manifest) { m.ReworkVersion = "latest" }},
		{"upstream", func(m *Manifest) { m.UpstreamVersion = "main" }},
		{"sha", func(m *Manifest) { m.GitSHA = "abc" }},
		{"migration", func(m *Manifest) { m.MigrationMin, m.MigrationMax = 234, 233 }},
		{"date", func(m *Manifest) { m.ReleaseDate = "today" }},
		{"compatibility", func(m *Manifest) { m.Compatibility = "unknown" }},
		{"updater", func(m *Manifest) { m.MinimumUpdaterVersion = "latest" }},
		{"notes", func(m *Manifest) { m.ReleaseNotes.Rollback = strings.Repeat("x", MaxNoteBytes+1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validManifest()
			test.mutate(&manifest)
			_, err := Parse(encodeManifest(t, manifest), trustedRepository)
			require.Error(t, err)
		})
	}
}

func TestOperationRequestHasNoPrivilegedParameters(t *testing.T) {
	data, err := json.Marshal(OperationRequest{Version: "0.1.184-rework.1", Confirmation: "INSTALL 0.1.184-rework.1", Actor: "admin:1"})
	require.NoError(t, err)
	require.JSONEq(t, `{"version":"0.1.184-rework.1","confirmation":"INSTALL 0.1.184-rework.1","actor":"admin:1"}`, string(data))
	require.NotContains(t, string(data), "image")
	require.NotContains(t, string(data), "command")
	require.NotContains(t, string(data), "path")
}
