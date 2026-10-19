//go:build integration

package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestClaudeFleetRefreshHTTPAndDatabase(t *testing.T) {
	ctx := t.Context()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	var mu sync.Mutex
	calls := make(map[string]int)
	reset := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/oauth/usage", r.URL.Path)
		key := r.Header.Get("Authorization")
		mu.Lock()
		calls[key]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if key == "Bearer fixture-C" {
			w.Header().Set("Retry-After", "600")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("fixture-secret-error-body"))
			return
		}
		percent := 25
		if key == "Bearer fixture-B" {
			percent = 0
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"five_hour": map[string]any{"utilization": percent, "resets_at": reset.Format(time.RFC3339)}})
	}))
	defer server.Close()
	var ids []int64
	for _, key := range []string{"fixture-A", "fixture-B", "fixture-C"} {
		a := mustCreateAccount(t, tx.Client(), &service.Account{Name: key, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
			Credentials: map[string]any{"access_token": key, "scope": "user:profile"}, Status: service.StatusActive})
		ids = append(ids, a.ID)
	}
	setup := mustCreateAccount(t, tx.Client(), &service.Account{Name: "fixture-setup", Platform: service.PlatformAnthropic, Type: service.AccountTypeSetupToken, Credentials: map[string]any{"access_token": "must-not-query"}})
	ids = append(ids, setup.ID)
	fetcher := &claudeUsageService{usageURL: server.URL + "/api/oauth/usage", allowPrivateHosts: true}
	svc := service.NewAccountUsageService(repo, NewUsageLogRepository(tx.Client(), integrationDB), fetcher, nil, nil, nil, nil, nil, service.NewUsageCache(), nil, nil)
	usage, failures, err := svc.GetUsageBatch(ctx, ids, true)
	require.NoError(t, err)
	require.Len(t, failures, 1)
	require.Contains(t, failures[ids[2]], "429")
	require.NotContains(t, failures[ids[2]], "fixture-secret")
	require.Equal(t, 25.0, usage[ids[0]].FiveHour.Utilization)
	require.Zero(t, usage[ids[1]].FiveHour.Utilization)
	require.Nil(t, usage[ids[1]].SevenDay)
	for _, id := range ids[:2] {
		a, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "active", a.Extra["session_window_utilization_source"])
		require.Equal(t, usage[id].FiveHour.ObservedAt.Format(time.RFC3339Nano), a.Extra["session_window_utilization_observed_at"])
		require.Equal(t, reset.Unix(), a.SessionWindowEnd.Unix())
	}
	observed := *usage[ids[0]].FiveHour.ObservedAt
	for range 3 {
		usage, failures, err = svc.GetUsageBatch(ctx, ids, false)
		require.NoError(t, err)
		require.Len(t, failures, 1)
		require.True(t, usage[ids[0]].Cached)
		require.Equal(t, observed, *usage[ids[0]].FiveHour.ObservedAt)
	}
	mu.Lock()
	require.Equal(t, map[string]int{"Bearer fixture-A": 1, "Bearer fixture-B": 1, "Bearer fixture-C": 1}, calls)
	mu.Unlock()
	_, failures, err = svc.GetUsageBatch(ctx, ids, true)
	require.NoError(t, err)
	require.Len(t, failures, 1)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, map[string]int{"Bearer fixture-A": 2, "Bearer fixture-B": 2, "Bearer fixture-C": 1}, calls, "manual refresh bypasses successful observations but never Retry-After")
}
