//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageObservationCompletenessPreservesChargesAndAttribution(t *testing.T) {
	var charged float64
	for _, complete := range []*bool{nil, boolPtr(false), boolPtr(true)} {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		users := &openAIRecordUsageUserRepoStub{}
		svc := newGatewayRecordUsageServiceForTest(repo, users, &openAIRecordUsageSubRepoStub{})
		err := svc.RecordUsage(t.Context(), &RecordUsageInput{
			Result: &ForwardResult{UsageObservationComplete: complete, RequestID: "observation-fixture", Model: "claude-sonnet-4",
				Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}},
			User: &User{ID: 601}, APIKey: &APIKey{ID: 501, Quota: 100}, Account: &Account{ID: 701},
		})
		require.NoError(t, err)
		require.Equal(t, complete, repo.lastLog.UsageObservationComplete)
		require.Equal(t, int64(601), repo.lastLog.UserID)
		require.Equal(t, int64(501), repo.lastLog.APIKeyID)
		require.Equal(t, int64(701), repo.lastLog.AccountID)
		require.Positive(t, repo.lastLog.ActualCost)
		if complete == nil {
			charged = repo.lastLog.ActualCost
		}
		require.Equal(t, charged, repo.lastLog.ActualCost)
		require.Equal(t, charged, users.lastAmount)
	}
}
