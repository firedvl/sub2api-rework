//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.1.183-rework.5", metadata.ReworkVersion)
	require.Equal(t, "v0.1.183", metadata.UpstreamBaseline)
	require.Equal(t, "e8cb019fabf8b55199436229044cbf9aa7a82564", metadata.UpstreamBaselineSHA)
	require.Equal(t, "ghcr.io/firedvl/sub2api-rework", metadata.ArtifactRepository)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.2", metadata.MinimumUpdaterVersion)
	require.Equal(t, 232, metadata.MigrationMin)
	require.Equal(t, 232, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
