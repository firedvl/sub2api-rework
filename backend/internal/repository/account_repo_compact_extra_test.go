package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestShouldEnqueueSchedulerOutboxForExtraUpdates_CompactCapabilityKeysAreRelevant(t *testing.T) {
	updates := map[string]any{
		"openai_compact_supported":  true,
		"openai_compact_checked_at": "2026-04-10T10:00:00Z",
	}

	if !shouldEnqueueSchedulerOutboxForExtraUpdates(updates) {
		t.Fatalf("expected compact capability updates to enqueue scheduler outbox")
	}
}

func TestShouldEnqueueSchedulerOutboxForExtraUpdates_OpenAIResponsesCapabilityKeysAreRelevant(t *testing.T) {
	updates := map[string]any{
		"openai_responses_mode":      "force_chat_completions",
		"openai_responses_supported": false,
	}

	if !shouldEnqueueSchedulerOutboxForExtraUpdates(updates) {
		t.Fatalf("expected responses capability updates to enqueue scheduler outbox")
	}
}

func TestShouldEnqueueSchedulerOutboxForExtraUpdates_VisionQualificationEvidenceIsNeutral(t *testing.T) {
	if shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		service.OpenAIVisionQualificationExtraKey: map[string]any{"state": "QUALIFIED"},
	}) {
		t.Fatal("qualification evidence must not enqueue scheduler work")
	}
}

func TestClaudeUsageObservationMetadataIsSchedulerNeutral(t *testing.T) {
	for _, key := range []string{
		"session_window_usage_reset", "session_window_utilization_source", "session_window_utilization_observed_at",
		"passive_usage_7d_utilization_source", "passive_usage_7d_utilization_observed_at",
		"passive_usage_7d_oi_utilization_source", "passive_usage_7d_oi_utilization_observed_at",
		"passive_usage_7d_sonnet_utilization", "passive_usage_7d_sonnet_reset", "passive_usage_7d_sonnet_utilization_source", "passive_usage_7d_sonnet_utilization_observed_at",
	} {
		if shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{key: "observation"}) {
			t.Errorf("metadata %s must not enqueue scheduler work", key)
		}
	}
	if !shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{"account_scheduling_threshold": true, "session_window_utilization_observed_at": "observation"}) {
		t.Fatal("real scheduling changes must still enqueue work")
	}
}
