//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type cnQuota403Repo struct {
	rateLimitAccountRepoStub
	rateLimitedAt  time.Time
	rateLimitCalls int
}

func (r *cnQuota403Repo) SetRateLimited(_ context.Context, _ int64, until time.Time) error {
	r.rateLimitCalls++
	r.rateLimitedAt = until
	return nil
}

func TestKimiCodingQuota403DoesNotDisableAccount(t *testing.T) {
	for _, withSnapshot := range []bool{true, false} {
		t.Run(map[bool]string{true: "snapshot", false: "no snapshot"}[withSnapshot], func(t *testing.T) {
			repo := &cnQuota403Repo{}
			counter := &openAI403CounterCacheStub{counts: []int64{3}}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			svc.SetOpenAI403CounterCache(counter)
			account := &Account{ID: 401, Platform: PlatformKimi, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"account_mode": AccountModeCoding}, Extra: map[string]any{}}
			if withSnapshot {
				account.Extra[cnExtraKey(PlatformKimi, cnExtraSuffix5hReset)] = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
				account.Extra[cnExtraKey(PlatformKimi, cnExtraSuffixWeeklyReset)] = time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
			}
			body := []byte(`{"error":{"message":"Your quota will reset when the current window ends.","type":"access_terminated_error"}}`)
			require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, body))
			require.Zero(t, repo.setErrorCalls)
			if withSnapshot {
				require.Equal(t, 1, repo.rateLimitCalls)
				require.Zero(t, repo.tempCalls)
				require.WithinDuration(t, time.Now().Add(2*time.Hour), repo.rateLimitedAt, time.Second)
			} else {
				require.Zero(t, repo.rateLimitCalls)
				require.Equal(t, 1, repo.tempCalls)
				require.Contains(t, repo.lastTempReason, cnQuotaExhaustedReasonPrefix)
			}
		})
	}
}

func TestCNQuota403ClassificationKeepsOther403Paths(t *testing.T) {
	body := []byte(`{"error":{"message":"Your quota will reset soon","type":"access_terminated_error"}}`)
	account := &Account{Platform: PlatformKimi, Credentials: map[string]any{"account_mode": AccountModeCoding}}
	require.True(t, isCNProviderQuotaExhausted403(account, body, "Your quota will reset soon"))
	require.True(t, isCNProviderQuotaExhausted403(account, []byte(`{"error":{"type":"access_terminated_error"}}`), ""))
	account.Credentials["account_mode"] = AccountModePayG
	require.False(t, isCNProviderQuotaExhausted403(account, body, "Your quota will reset soon"))
	repo := &cnQuota403Repo{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetOpenAI403CounterCache(&openAI403CounterCacheStub{counts: []int64{3}})
	account.ID = 402
	require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, body))
	require.Equal(t, 1, repo.setErrorCalls)
	require.Zero(t, repo.rateLimitCalls)
	account.Platform = PlatformOpenAI
	require.False(t, isCNProviderQuotaExhausted403(account, body, "Your quota will reset soon"))
	require.False(t, isCNProviderQuotaExhausted403(nil, body, "Your quota will reset soon"))
}
