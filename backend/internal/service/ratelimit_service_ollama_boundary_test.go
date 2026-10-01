//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ollamaBoundaryRepository struct {
	mockAccountRepoForGemini
	resetAt time.Time
}

func (repository *ollamaBoundaryRepository) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	repository.resetAt = resetAt
	return nil
}

func (repository *ollamaBoundaryRepository) RecordOllamaCloudUsage429(_ context.Context, observed *Account, resetAt *time.Time) (*AccountRateLimitGeneration, error) {
	if resetAt != nil && resetAt.After(repository.resetAt) {
		repository.resetAt = *resetAt
		account := repository.accountsByID[observed.ID]
		account.RateLimitResetAt = cloneTimePtr(resetAt)
		limitedAt := time.Now()
		account.RateLimitedAt = &limitedAt
	}
	account := repository.accountsByID[observed.ID]
	return &AccountRateLimitGeneration{LimitedAt: *account.RateLimitedAt, ResetAt: cloneTimePtr(account.RateLimitResetAt)}, nil
}

func (*ollamaBoundaryRepository) SetRateLimitedIfUnchanged(context.Context, int64, time.Time, *time.Time, *time.Time, time.Time) (bool, error) {
	panic("unexpected async callback in boundary test")
}

func TestOllama429DoesNotInterpretForeignCodexQuotaHeaders(t *testing.T) {
	account := &Account{ID: 99, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"base_url": "https://ollama.com", "api_key": "fixture-key"}}
	repository := &ollamaBoundaryRepository{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}}
	service := NewRateLimitService(repository, nil, nil, nil, nil)
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "7200")
	headers.Set("x-codex-primary-window-minutes", "300")
	before := time.Now()
	service.handle429(context.Background(), account, headers, nil)
	require.True(t, repository.resetAt.After(before))
	require.True(t, repository.resetAt.Before(before.Add(time.Minute)), "Ollama 429 must use its own seconds cooldown, not foreign Codex windows")
	require.Empty(t, account.Extra, "foreign Codex quota must not become Ollama account state")
}
