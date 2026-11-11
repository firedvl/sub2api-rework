package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelCatalogSyncRepo struct {
	AccountRepository
	accounts []Account
	updates  []int64
}

func (r *modelCatalogSyncRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, _ []string, includeGrouped bool) ([]Account, error) {
	if groupID != nil || !includeGrouped {
		return nil, errors.New("sync must include accounts in all groups")
	}
	return append([]Account(nil), r.accounts...), nil
}

func (r *modelCatalogSyncRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.updates = append(r.updates, id)
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			if r.accounts[i].Extra == nil {
				r.accounts[i].Extra = make(map[string]any)
			}
			for key, value := range updates {
				r.accounts[i].Extra[key] = value
			}
		}
	}
	return nil
}

func modelCatalogSyncBody(model string) string {
	body, _ := json.Marshal(map[string]any{"data": []map[string]any{{
		"id": model, "reasoning": false, "input_modalities": []string{"text"}, "context_window": 100000,
	}}})
	return string(body)
}

func TestModelCatalogSyncRefreshesStaleAccountsAndPreservesFailure(t *testing.T) {
	repo := &modelCatalogSyncRepo{accounts: []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "synthetic"}},
		{ID: 2, Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic"}},
		{ID: 3, Platform: PlatformTypeSafe, Type: AccountTypeAPIKey},
		{ID: 4, Platform: PlatformAnthropic, Type: AccountTypeBedrock},
	}}
	old := UpstreamModelInventorySnapshot{Source: "upstream", SyncedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339), Models: []string{"old-model"}}
	repo.accounts[1].SetUpstreamModelInventorySnapshot(old)
	upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, id int64, _ int) (*http.Response, error) {
		if id == 2 {
			return nil, errors.New("temporary failure")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(modelCatalogSyncBody("claude-fable-5-2")))}, nil
	}}
	svc := NewModelCatalogSyncService(repo, &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}, nil)
	defer svc.Stop()
	svc.runOnce()
	require.Equal(t, []int64{1}, repo.updates)
	require.Equal(t, []string{"claude-fable-5-2"}, repo.accounts[0].CatalogModelIDs())
	require.Equal(t, old, *repo.accounts[1].GetUpstreamModelInventorySnapshot())
	svc.runOnce()
	require.Equal(t, []int64{1}, repo.updates, "fresh inventory must not be fetched again")
	repo.accounts[0].SetUpstreamModelInventorySnapshot(old)
	svc.runOnce()
	require.Equal(t, []int64{1, 1}, repo.updates, "stale inventory must refresh on later cycles")
}

func TestModelCatalogSyncStopsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	repo := &modelCatalogSyncRepo{accounts: []Account{{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "synthetic"}}}}
	upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		close(started)
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	svc := NewModelCatalogSyncService(repo, &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}, nil)
	svc.Start()
	svc.Start()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial sync did not start")
	}
	done := make(chan struct{})
	go func() { svc.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel model discovery")
	}
	svc.Stop()
}
