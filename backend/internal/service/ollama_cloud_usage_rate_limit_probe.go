package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

const (
	ollamaCloudUsageProbeTimeout = 45 * time.Second

	ollamaCloudUsageProbeMaxQueue = 256

	ollamaCloudUsageProbeGroupRetention = 2 * ollamaCloudUsageManualRefreshInterval
)

type OllamaCloudUsageRateLimitProbeCallback func(accountID int64, resetAt time.Time)

type ollamaCloudUsageProbeRequest struct {
	accountID   int64
	onExhausted OllamaCloudUsageRateLimitProbeCallback
}

type ollamaCloudUsageProbeGroupEntry struct {
	attemptAt time.Time
	snapshot  *OllamaCloudUsageSnapshot
}

func (service *OllamaCloudUsageService) ScheduleOllamaCloudUsageRateLimitProbe(
	accountID int64,
	onExhausted OllamaCloudUsageRateLimitProbeCallback,
) bool {
	if service == nil || onExhausted == nil || accountID <= 0 {
		return false
	}
	service.mu.Lock()
	running := service.started && !service.stopped
	service.mu.Unlock()
	if !running {
		return false
	}

	service.probeMu.Lock()
	for index := range service.probeQueue {
		if service.probeQueue[index].accountID == accountID {
			service.probeQueue[index].onExhausted = onExhausted
			service.probeMu.Unlock()
			service.wakeProbeLoop()
			return true
		}
	}
	if len(service.probeQueue) >= ollamaCloudUsageProbeMaxQueue {
		service.probeMu.Unlock()
		return false
	}
	service.probeQueue = append(service.probeQueue, ollamaCloudUsageProbeRequest{
		accountID:   accountID,
		onExhausted: onExhausted,
	})
	service.probeMu.Unlock()

	service.wakeProbeLoop()
	return true
}

func (service *OllamaCloudUsageService) wakeProbeLoop() {
	if service == nil {
		return
	}
	select {
	case service.probeWake <- struct{}{}:
	default:
	}
}

func (service *OllamaCloudUsageService) probeLoop() {
	defer service.wg.Done()
	for {
		service.probeMu.Lock()
		if len(service.probeQueue) == 0 {
			service.probeMu.Unlock()
			select {
			case <-service.probeWake:
			case <-service.parentCtx.Done():
				return
			}
			continue
		}
		batch := service.probeQueue
		service.probeQueue = nil
		service.probeMu.Unlock()

		for _, req := range batch {
			if service.parentCtx.Err() != nil {
				return
			}
			ctx, cancel := context.WithTimeout(service.parentCtx, ollamaCloudUsageProbeTimeout)
			service.runOllamaCloudUsageProbe(ctx, req.accountID, req.onExhausted)
			cancel()
		}
	}
}

func (service *OllamaCloudUsageService) probeGroupResult(key string) (ollamaCloudUsageProbeGroupEntry, bool) {
	if service == nil {
		return ollamaCloudUsageProbeGroupEntry{}, false
	}
	service.probeMu.Lock()
	defer service.probeMu.Unlock()
	entry, ok := service.probeGroups[key]
	return entry, ok
}

func (service *OllamaCloudUsageService) storeProbeGroupResult(key string, entry ollamaCloudUsageProbeGroupEntry) {
	if service == nil {
		return
	}
	service.probeMu.Lock()
	defer service.probeMu.Unlock()
	if service.probeGroups == nil {
		service.probeGroups = make(map[string]ollamaCloudUsageProbeGroupEntry)
	}
	pruneAt := entry.attemptAt.Add(-ollamaCloudUsageProbeGroupRetention)
	for groupKey, existing := range service.probeGroups {
		if existing.attemptAt.Before(pruneAt) {
			delete(service.probeGroups, groupKey)
		}
	}
	service.probeGroups[key] = entry
}

func (service *OllamaCloudUsageService) runOllamaCloudUsageProbe(
	ctx context.Context,
	accountID int64,
	onExhausted OllamaCloudUsageRateLimitProbeCallback,
) {
	if service == nil || service.accountRepo == nil || onExhausted == nil || ctx.Err() != nil {
		return
	}
	account, err := service.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		if err != nil && ctx.Err() == nil {
			slog.Warn("ollama_cloud_usage_probe_load_failed", "account_id", accountID, "error", logredact.RedactText(err.Error()))
		}
		return
	}
	if !IsOllamaCloudUsageAccount(account) {
		return
	}
	if err := service.ResolveAccounts(ctx, []*Account{account}); err != nil {
		if ctx.Err() == nil {
			slog.Warn("ollama_cloud_usage_probe_resolve_failed", "account_id", accountID, "error", logredact.RedactText(err.Error()))
		}
		return
	}
	if !ollamaCloudUsageConfigured(account) {
		return
	}
	key, valid := ollamaCloudUsageRateLimitFingerprint(account)
	if !valid {
		return
	}

	window := ollamaCloudUsageManualRefreshInterval
	accountSnapshot := decodeOllamaCloudUsageSnapshot(account.Extra)
	cached, hasCached := service.probeGroupResult(key)

	now := service.currentTime()
	if newest := ollamaCloudUsageProbeNewestSuccess(accountSnapshot, cached, hasCached, now, window); newest != nil {
		maybeOllamaCloudUsageProbeExhaustion(ctx, accountID, newest, service.currentTime(), window, onExhausted)
		return
	}

	if horizon := ollamaCloudUsageProbeBackoffHorizon(accountSnapshot, cached, hasCached); !horizon.IsZero() && now.Before(horizon) {
		return
	}
	if hasCached && now.Before(cached.attemptAt.Add(window)) {
		return
	}

	settings, settingsErr := service.GetSettings(ctx)
	if settingsErr != nil {
		slog.Warn("ollama_cloud_usage_probe_settings_failed", "account_id", accountID, "error", logredact.RedactText(settingsErr.Error()))
		return
	}
	fetched, refreshErr := service.refreshAccount(ctx, accountID, settings, false)
	if refreshErr == nil && fetched == nil {
		return
	}
	doneNow := service.currentTime()
	service.storeProbeGroupResult(key, ollamaCloudUsageProbeGroupEntry{attemptAt: doneNow, snapshot: fetched})
	if refreshErr != nil {
		if ctx.Err() == nil {
			slog.Warn("ollama_cloud_usage_probe_refresh_failed", "account_id", accountID, "error", logredact.RedactText(refreshErr.Error()))
		}
		return
	}
	maybeOllamaCloudUsageProbeExhaustion(ctx, accountID, fetched, doneNow, window, onExhausted)
}

func ollamaCloudUsageProbeObservedAt(snapshot *OllamaCloudUsageSnapshot) (time.Time, bool) {
	if snapshot == nil {
		return time.Time{}, false
	}
	if snapshot.Status == OllamaCloudUsageStatusOK && snapshot.FetchedAt != nil && !snapshot.FetchedAt.IsZero() {
		return snapshot.FetchedAt.UTC(), true
	}
	if !snapshot.LastAttemptAt.IsZero() {
		return snapshot.LastAttemptAt.UTC(), true
	}
	return time.Time{}, false
}

func ollamaCloudUsageProbeNewestSuccess(
	accountSnapshot *OllamaCloudUsageSnapshot,
	cached ollamaCloudUsageProbeGroupEntry,
	hasCached bool,
	now time.Time,
	window time.Duration,
) *OllamaCloudUsageSnapshot {
	var newest *OllamaCloudUsageSnapshot
	var newestAt time.Time
	consider := func(snapshot *OllamaCloudUsageSnapshot) {
		if snapshot == nil {
			return
		}
		at, ok := ollamaCloudUsageProbeObservedAt(snapshot)
		if !ok {
			return
		}
		if newest == nil || at.After(newestAt) {
			newest = snapshot
			newestAt = at
		}
	}
	consider(accountSnapshot)
	if hasCached {
		if cached.snapshot != nil {
			consider(cached.snapshot)
		} else if !cached.attemptAt.IsZero() && (newest == nil || cached.attemptAt.After(newestAt)) {
			newest = nil
			newestAt = cached.attemptAt
		}
	}
	if newest == nil || newest.Status != OllamaCloudUsageStatusOK {
		return nil
	}
	if newest.FetchedAt == nil || newest.FetchedAt.IsZero() || newest.FetchedAt.Before(now.Add(-window)) {
		return nil
	}
	return newest
}

func ollamaCloudUsageProbeBackoffHorizon(
	accountSnapshot *OllamaCloudUsageSnapshot,
	cached ollamaCloudUsageProbeGroupEntry,
	hasCached bool,
) time.Time {
	var horizon time.Time
	consider := func(snapshot *OllamaCloudUsageSnapshot) {
		if snapshot == nil {
			return
		}
		if snapshot.Status != OllamaCloudUsageStatusFailed && snapshot.Status != OllamaCloudUsageStatusUnauthorized {
			return
		}
		if snapshot.NextRefreshAt.IsZero() {
			return
		}
		if horizon.IsZero() || snapshot.NextRefreshAt.After(horizon) {
			horizon = snapshot.NextRefreshAt.UTC()
		}
	}
	consider(accountSnapshot)
	if hasCached {
		consider(cached.snapshot)
	}
	return horizon
}

func maybeOllamaCloudUsageProbeExhaustion(
	ctx context.Context,
	accountID int64,
	snapshot *OllamaCloudUsageSnapshot,
	now time.Time,
	window time.Duration,
	onExhausted OllamaCloudUsageRateLimitProbeCallback,
) {
	if snapshot == nil || onExhausted == nil {
		return
	}
	resetAt, exhausted := ollamaCloudUsageExhaustionResetAt(snapshot, now, now.Add(-window))
	if !exhausted {
		return
	}
	if ctx.Err() != nil {
		return
	}
	onExhausted(accountID, resetAt)
}
