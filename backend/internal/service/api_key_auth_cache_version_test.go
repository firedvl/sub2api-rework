package service

import (
	"encoding/json"
	"testing"
)

func TestAPIKeyService_RejectsV10AuthSnapshotWithoutModelsListConfig(t *testing.T) {
	groupID := int64(9)
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-models-list", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version:  10,
			APIKeyID: 1,
			UserID:   2,
			GroupID:  &groupID,
			Status:   StatusActive,
			User: APIKeyAuthUserSnapshot{
				ID:          2,
				Status:      StatusActive,
				Role:        RoleUser,
				Balance:     10,
				Concurrency: 3,
			},
			Group: &APIKeyAuthGroupSnapshot{
				ID:               groupID,
				Name:             "openai",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1,
			},
		},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatalf("expected v10 auth snapshot to be rejected after models_list_config was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_RejectsV15AuthSnapshotWithoutReasoningEffortPolicy(t *testing.T) {
	svc := &APIKeyService{}

	apiKey, ok, err := svc.applyAuthCacheEntry("k-legacy-reasoning-mappings", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 15},
	})

	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if ok {
		t.Fatal("expected v15 auth snapshot to be rejected after reasoning effort policy was added")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_RejectsV20AuthSnapshotWithoutRestrictPublicGroups(t *testing.T) {
	const legacyV20JSON = `{
		"snapshot": {
			"version": 20,
			"api_key_id": 1,
			"user_id": 2,
			"status": "active",
			"user": {
				"id": 2,
				"status": "active",
				"role": "user",
				"allowed_groups": [9]
			}
		}
	}`

	var entry APIKeyAuthCacheEntry
	if err := json.Unmarshal([]byte(legacyV20JSON), &entry); err != nil {
		t.Fatalf("unmarshal legacy v20 auth snapshot: %v", err)
	}
	if entry.Snapshot.User.RestrictPublicGroups {
		t.Fatal("legacy snapshot unexpectedly contains a public-group restriction")
	}

	apiKey, used, err := (&APIKeyService{}).applyAuthCacheEntry("k-legacy-public-groups", &entry)
	if err != nil {
		t.Fatalf("expected stale snapshot to be ignored without error, got %v", err)
	}
	if used {
		t.Fatal("expected v20 auth snapshot to be rejected before authorization")
	}
	if apiKey != nil {
		t.Fatalf("expected no API key from stale snapshot, got %#v", apiKey)
	}
}

func TestAPIKeyService_AcceptsV21AuthSnapshotWithRestrictPublicGroups(t *testing.T) {
	entry := &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{
		Version:  apiKeyAuthSnapshotVersion,
		APIKeyID: 1,
		UserID:   2,
		Status:   StatusActive,
		User: APIKeyAuthUserSnapshot{
			ID:                   2,
			Status:               StatusActive,
			Role:                 RoleUser,
			AllowedGroups:        []int64{9},
			RestrictPublicGroups: true,
		},
	}}

	apiKey, used, err := (&APIKeyService{}).applyAuthCacheEntry("k-current-public-groups", entry)
	if err != nil {
		t.Fatalf("apply current auth snapshot: %v", err)
	}
	if !used {
		t.Fatal("expected current auth snapshot to be accepted")
	}
	if apiKey == nil || apiKey.User == nil || !apiKey.User.RestrictPublicGroups {
		t.Fatalf("expected public-group restriction to survive authorization materialization, got %#v", apiKey)
	}
	if apiKey.User.CanBindGroup(10, false) {
		t.Fatal("restricted user unexpectedly authorized for an unlisted public group")
	}
}
