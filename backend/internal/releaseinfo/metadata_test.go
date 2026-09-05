//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.2.0-rework.3", metadata.ReworkVersion)
	require.Equal(t, "v0.2.0", metadata.UpstreamBaseline)
	require.Equal(t, "aa236488351eb71e120fc2b6fb32e36b0374c918", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.3", metadata.MinimumUpdaterVersion)
	require.Equal(t, 235, metadata.MigrationMin)
	require.Equal(t, 239, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
