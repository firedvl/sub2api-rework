//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImagesBalanceCooldownKeepsLaterDurableReset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	account := mustCreateAccount(t, client, &service.Account{
		Name: "image-balance-cooldown", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
	})
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	initial := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	require.NoError(t, repo.SetModelRateLimit(ctx, account.ID, "openai:image_generation", initial, "openai_images_insufficient_balance"))
	got, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	limit := got.Extra["model_rate_limits"].(map[string]any)["openai:image_generation"].(map[string]any)
	require.Equal(t, initial.Format(time.RFC3339), limit["rate_limit_reset_at"])
	require.Equal(t, "openai_images_insufficient_balance", limit["reason"])

	later := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, repo.SetModelRateLimit(ctx, account.ID, "openai:image_generation", later, "image_429"))
	require.NoError(t, repo.SetModelRateLimit(ctx, account.ID, "openai:image_generation", time.Now().Add(5*time.Minute), "openai_images_insufficient_balance"))
	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	limit = got.Extra["model_rate_limits"].(map[string]any)["openai:image_generation"].(map[string]any)
	require.Equal(t, later.Format(time.RFC3339), limit["rate_limit_reset_at"])
	require.Equal(t, "image_429", limit["reason"])

	offset := time.FixedZone("PDT", -7*3600)
	offsetReset := time.Now().Add(2 * time.Hour).In(offset).Truncate(time.Second)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra = jsonb_set(extra,
		ARRAY['model_rate_limits', 'openai:image_generation', 'rate_limit_reset_at']::text[],
		to_jsonb($1::text), true) WHERE id = $2`, offsetReset.Format(time.RFC3339), account.ID)
	require.NoError(t, err)
	cache := &schedulerCacheRecorder{accounts: map[int64]*service.Account{account.ID: {ID: account.ID}}}
	repo.schedulerCache = cache
	require.NoError(t, repo.SetModelRateLimit(ctx, account.ID, "openai:image_generation", time.Now().Add(time.Hour), "openai_images_insufficient_balance"))
	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	limit = got.Extra["model_rate_limits"].(map[string]any)["openai:image_generation"].(map[string]any)
	require.Equal(t, offsetReset.Format(time.RFC3339), limit["rate_limit_reset_at"])
	require.Equal(t, "image_429", limit["reason"])
	require.NotEmpty(t, cache.setAccounts)
	cached := cache.accounts[account.ID].Extra["model_rate_limits"].(map[string]any)["openai:image_generation"].(map[string]any)
	require.Equal(t, offsetReset.Format(time.RFC3339), cached["rate_limit_reset_at"])
}
