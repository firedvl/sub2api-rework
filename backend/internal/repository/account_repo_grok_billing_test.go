package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGrokBillingSnapshotIsSchedulerNeutral(t *testing.T) {
	t.Parallel()

	require.True(t, isSchedulerNeutralExtraKey("grok_billing_snapshot"))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		"grok_billing_snapshot": map[string]any{"usage_percent": 50},
	}))
}

func TestOpenAICodexManifestSnapshotIsSchedulerNeutral(t *testing.T) {
	t.Parallel()

	require.True(t, isSchedulerNeutralExtraKey(service.OpenAICodexManifestSnapshotExtraKey))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		service.OpenAICodexManifestSnapshotExtraKey: map[string]any{"body": map[string]any{"models": []any{}}},
	}))
}
