package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const modelCatalogSyncInterval = time.Hour

// ModelCatalogSyncService refreshes provider inventories without changing account
// mappings or group allowlists. Failed requests leave the last snapshot intact.
type ModelCatalogSyncService struct {
	accountRepo AccountRepository
	accountTest *AccountTestService
	gateway     *GatewayService
	ctx         context.Context
	cancel      context.CancelFunc
	startOnce   sync.Once
	wg          sync.WaitGroup
}

func NewModelCatalogSyncService(repo AccountRepository, accountTest *AccountTestService, gateway *GatewayService) *ModelCatalogSyncService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModelCatalogSyncService{accountRepo: repo, accountTest: accountTest, gateway: gateway, ctx: ctx, cancel: cancel}
}

func (s *ModelCatalogSyncService) Start() {
	if s == nil || s.accountRepo == nil || s.accountTest == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runOnce()
			ticker := time.NewTicker(modelCatalogSyncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.runOnce()
					// Start the interval after persistence so the freshness check permits it.
					ticker.Reset(modelCatalogSyncInterval)
				case <-s.ctx.Done():
					return
				}
			}
		}()
	})
}

func (s *ModelCatalogSyncService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

func (s *ModelCatalogSyncService) runOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, nil, gatewayCapabilityPlatforms, true)
	cancel()
	if err != nil {
		slog.Warn("model_catalog_sync_list_failed")
		return
	}
	for i := range accounts {
		if s.ctx.Err() != nil {
			return
		}
		account := &accounts[i]
		if account.IsTypeSafe() || account.IsBedrock() || account.Type == AccountTypeServiceAccount {
			continue
		}
		if inventory := account.GetUpstreamModelInventorySnapshot(); inventory != nil {
			if syncedAt, err := time.Parse(time.RFC3339, inventory.SyncedAt); err == nil && time.Since(syncedAt) >= 0 && time.Since(syncedAt) < modelCatalogSyncInterval {
				continue
			}
		}
		ctx, cancel := context.WithTimeout(s.ctx, time.Minute)
		catalog, err := s.accountTest.SyncUpstreamModelCatalog(ctx, account)
		cancel()
		if err != nil {
			var syncErr *UpstreamModelSyncError
			if errors.As(err, &syncErr) {
				if syncErr.Kind == UpstreamModelSyncErrorUnsupported {
					continue
				}
				slog.Warn("model_catalog_sync_failed", "account_id", account.ID, "platform", account.Platform, "kind", syncErr.Kind, "status_code", syncErr.StatusCode)
			} else {
				slog.Warn("model_catalog_sync_failed", "account_id", account.ID, "platform", account.Platform)
			}
			continue
		}
		if s.gateway != nil {
			s.gateway.InvalidateAvailableModelsCache(nil, "")
		}
		slog.Info("model_catalog_synced", "account_id", account.ID, "platform", account.Platform, "model_count", len(catalog.Models))
	}
}
