package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexMaximumContextMetadataIsConservative(t *testing.T) {
	for _, tc := range []struct {
		name       string
		candidates []UpstreamModelMetadata
		want       int64
	}{
		{"explicit maximum", []UpstreamModelMetadata{{ContextWindow: 100000, MaxContextWindow: 900000}}, 900000},
		{"mixed maxima", []UpstreamModelMetadata{{ContextWindow: 100000, MaxContextWindow: 900000}, {ContextWindow: 100000, MaxContextWindow: 500000}}, 500000},
		{"older snapshot", []UpstreamModelMetadata{{ContextWindow: 100000, MaxContextWindow: 900000}, {ContextWindow: 64000}}, 64000},
		{"unknown ceiling", []UpstreamModelMetadata{{ContextWindow: 100000, MaxContextWindow: 900000}, {}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := intersectUpstreamModelMetadata("custom-model", tc.candidates)
			require.Equal(t, tc.want, metadata.MaxContextWindow)
			if tc.want > 0 {
				descriptor := newConfiguredCodexModelDescriptor("custom-model")
				applyUpstreamModelMetadataToCodexDescriptor(&descriptor, metadata)
				require.Equal(t, tc.want, descriptor.MaxContextWindow)
				require.LessOrEqual(t, descriptor.ContextWindow, descriptor.MaxContextWindow)
			}
		})
	}
}

func TestUpstreamContextDefaultDoesNotInventMaximum(t *testing.T) {
	merged, _ := mergeUpstreamModelMetadata(UpstreamModelMetadata{ContextWindow: 64000}, UpstreamModelMetadata{ContextWindow: 100000, MaxContextWindow: 900000})
	require.Zero(t, merged.MaxContextWindow)
	metadata := upstreamMetadataFromCapabilityEntry("custom-model", upstreamModelCapabilityEntry{ContextWindow: 100000, MaxContextWindow: 900000})
	require.EqualValues(t, 100000, metadata.ContextWindow)
	require.EqualValues(t, 900000, metadata.MaxContextWindow)
}
