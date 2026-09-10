//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.2.3-rework.1", metadata.ReworkVersion)
	require.Equal(t, "v0.2.3", metadata.UpstreamBaseline)
	require.Equal(t, "8fa67d477d6651a744754392a8982ea589c26ae6", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.4", metadata.MinimumUpdaterVersion)
	require.Equal(t, 239, metadata.MigrationMin)
	require.Equal(t, 244, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
