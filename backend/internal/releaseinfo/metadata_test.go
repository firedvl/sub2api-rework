//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.2.3-rework.7", metadata.ReworkVersion)
	require.Equal(t, "v0.2.8", metadata.UpstreamBaseline)
	require.Equal(t, "fd80b08c90b55edcad5b00171b53f08721d30da1", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.5", metadata.MinimumUpdaterVersion)
	require.Equal(t, 239, metadata.MigrationMin)
	require.Equal(t, 251, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
