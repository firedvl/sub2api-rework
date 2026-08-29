//go:build unit

package releaseinfo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMetadataIsCanonicalAndManual(t *testing.T) {
	metadata := Current()
	require.Equal(t, "0.1.183-rework.3", metadata.ReworkVersion)
	require.Equal(t, "v0.1.183", metadata.UpstreamBaseline)
	require.Equal(t, "manual", metadata.DefaultPolicy)
	require.Equal(t, "1.1.0", metadata.MinimumUpdaterVersion)
	require.Equal(t, 232, metadata.MigrationMax)

	var fromJSON Metadata
	require.NoError(t, json.Unmarshal(JSON(), &fromJSON))
	require.Equal(t, metadata, fromJSON)
}
