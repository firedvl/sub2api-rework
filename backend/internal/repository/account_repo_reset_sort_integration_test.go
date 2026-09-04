//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountListResetSoonestSorting(t *testing.T) {
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	now := time.Now().UTC()

	create := func(name string, fiveHour, weekly any) *service.Account {
		extra := map[string]any{}
		if fiveHour != nil {
			extra["codex_5h_reset_at"] = fiveHour
		}
		if weekly != nil {
			extra["codex_7d_reset_at"] = weekly
		}
		return mustCreateAccount(t, tx.Client(), &service.Account{Name: name, Extra: extra})
	}

	soon := create("reset-sort-a", now.Add(10*time.Minute).Format(time.RFC3339), now.Add(10*time.Minute).Format(time.RFC3339))
	equal := create("reset-sort-equal", now.Add(10*time.Minute).Format(time.RFC3339), now.Add(10*time.Minute).Format(time.RFC3339))
	later := create("reset-sort-b", now.Add(2*time.Hour).Format(time.RFC3339), now.Add(2*time.Hour).Format(time.RFC3339))
	tomorrow := create("reset-sort-c", now.Add(24*time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339))
	unknown := create("reset-sort-d", nil, nil)
	invalid := create("reset-sort-invalid", "not-a-time", "2026-02-30T00:00:00Z")
	stale := create("reset-sort-stale", now.Add(-time.Hour).Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339))
	ids := map[int64]struct{}{soon.ID: {}, equal.ID: {}, later.ID: {}, tomorrow.ID: {}, unknown.ID: {}, invalid.ID: {}, stale.ID: {}}

	assertOrder := func(sortBy string) {
		accounts, _, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{
			Page: 1, PageSize: 20, SortBy: sortBy, SortOrder: pagination.SortOrderAsc,
		}, "", "", "", "reset-sort-", 0, "")
		require.NoError(t, err)
		filtered := make([]int64, 0, len(accounts))
		for _, account := range accounts {
			if _, ok := ids[account.ID]; ok {
				filtered = append(filtered, account.ID)
			}
		}
		require.Equal(t, []int64{soon.ID, equal.ID, later.ID, tomorrow.ID, unknown.ID, invalid.ID, stale.ID}, filtered)

		page, _, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{
			Page: 2, PageSize: 2, SortBy: sortBy, SortOrder: pagination.SortOrderAsc,
		}, "", "", "", "reset-sort-", 0, "")
		require.NoError(t, err)
		require.Equal(t, []int64{later.ID, tomorrow.ID}, []int64{page[0].ID, page[1].ID})
	}

	assertOrder("reset_5h_at")
	assertOrder("reset_7d_at")
}
