//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.2.3-rework.8", metadata.ReworkVersion)
	require.Equal(t, "v0.2.14", metadata.UpstreamBaseline)
	require.Equal(t, "0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.6", metadata.MinimumUpdaterVersion)
	require.Equal(t, 239, metadata.MigrationMin)
	require.Equal(t, 251, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
