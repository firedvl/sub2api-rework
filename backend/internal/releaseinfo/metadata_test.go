//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.1.184-rework.4", metadata.ReworkVersion)
	require.Equal(t, "v0.1.184", metadata.UpstreamBaseline)
	require.Equal(t, "e98ef32eb29aecd30d1def615912ec4dc93173f3", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.3", metadata.MinimumUpdaterVersion)
	require.Equal(t, 232, metadata.MigrationMin)
	require.Equal(t, 235, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
