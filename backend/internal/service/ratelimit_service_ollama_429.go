package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const ollamaCloudUsageProbeWritebackTimeout = 10 * time.Second

type ollamaCloudUsageProbeScheduler interface {
	ScheduleOllamaCloudUsageRateLimitProbe(accountID int64, onExhausted OllamaCloudUsageRateLimitProbeCallback) bool
}

type ollamaCloudUsageRateLimitSetterIfGeneration interface {
	RecordOllamaCloudUsage429(ctx context.Context, observed *Account, resetAt *time.Time) (bool, error)
	SetRateLimitedIfUnchanged(ctx context.Context, id int64, expectedUpdatedAt time.Time, expectedLimitedAt, expectedResetAt *time.Time, newResetAt time.Time) (bool, error)
}

func (service *RateLimitService) SetOllamaCloudUsageProbeScheduler(scheduler ollamaCloudUsageProbeScheduler) {
	service.ollamaCloudUsageProbe = scheduler
}

func (service *RateLimitService) handleOllamaCloudUsage429(ctx context.Context, account *Account, headers http.Header) {
	if service == nil || account == nil || account.ID <= 0 || service.accountRepo == nil {
		return
	}
	originFingerprint, valid := ollamaCloudUsageRateLimitFingerprint(account)
	if !valid {
		return
	}
	observed, err := service.accountRepo.GetByID(ctx, account.ID)
	if err != nil || observed == nil {
		slog.Warn("ollama_cloud_usage_trigger_load_failed", "account_id", account.ID, "error", err)
		return
	}
	currentFingerprint, valid := ollamaCloudUsageRateLimitFingerprint(observed)
	if !valid || currentFingerprint != originFingerprint || !observed.IsActive() || !observed.Schedulable {
		slog.Debug("ollama_cloud_usage_trigger_identity_changed", "account_id", account.ID)
		return
	}

	var shortReset time.Time
	now := time.Now()
	if delay := retryAfter(headers, now); delay > 0 {
		shortReset = now.Add(delay)
	} else if cooldown, enabled := service.get429FallbackCooldown(ctx, account); enabled {
		shortReset = now.Add(cooldown)
	} else {
		slog.Info("rate_limit_ollama_429_fallback_ignored", "account_id", account.ID, "platform", account.Platform)
	}

	var resetAt *time.Time
	if !shortReset.IsZero() {
		resetAt = &shortReset
	}
	repository, supported := service.accountRepo.(ollamaCloudUsageRateLimitSetterIfGeneration)
	if !supported {
		slog.Error("ollama_rate_limit_generation_persistence_unavailable", "account_id", account.ID)
		return
	}
	updated, err := repository.RecordOllamaCloudUsage429(ctx, observed, resetAt)
	if err != nil {
		slog.Warn("ollama_rate_limit_trigger_persistence_failed", "account_id", account.ID, "error", err)
		return
	}
	if !updated {
		slog.Debug("ollama_rate_limit_trigger_skipped_stale", "account_id", account.ID)
		return
	}

	authoritative, err := service.accountRepo.GetByID(ctx, account.ID)
	if err != nil || authoritative == nil {
		slog.Warn("ollama_cloud_usage_authoritative_load_failed", "account_id", account.ID, "error", err)
		return
	}
	if authoritative.RateLimitResetAt != nil && authoritative.RateLimitResetAt.After(now) {
		service.notifyAccountSchedulingBlocked(authoritative, *authoritative.RateLimitResetAt, "ollama_429")
	}

	if service.ollamaCloudUsageProbe == nil {
		slog.Warn("ollama_rate_limit_probe_unavailable", "account_id", account.ID)
		return
	}
	service.scheduleOllamaCloudUsageProbe(authoritative)
}

func ollamaCloudUsageRateLimitFingerprint(account *Account) (string, bool) {
	group, valid := ollamaCloudUsageGroupFingerprint(account)
	if !valid {
		return "", false
	}
	proxy := "none"
	if account.ProxyID != nil {
		proxy = strconv.FormatInt(*account.ProxyID, 10)
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	session, _ := account.Extra[OllamaCloudUsageSessionExtraKey].(string)
	hash := sha256.Sum256([]byte(group + "\x00" + proxy + "\x00" + proxyURL + "\x00" + session))
	return hex.EncodeToString(hash[:]), true
}

func (service *RateLimitService) scheduleOllamaCloudUsageProbe(account *Account) {
	if service == nil || account == nil || service.ollamaCloudUsageProbe == nil {
		return
	}
	if !IsOllamaCloudUsageAccount(account) {
		return
	}
	fingerprint, valid := ollamaCloudUsageRateLimitFingerprint(account)
	if !valid {
		return
	}
	expectedLimitedAt := cloneTimePtr(account.RateLimitedAt)
	expectedResetAt := cloneTimePtr(account.RateLimitResetAt)
	accepted := service.ollamaCloudUsageProbe.ScheduleOllamaCloudUsageRateLimitProbe(
		account.ID,
		func(accountID int64, resetAt time.Time) {
			service.applyOllamaCloudUsageProbeReset(accountID, fingerprint, expectedLimitedAt, expectedResetAt, resetAt)
		},
	)
	if !accepted {
		slog.Debug("ollama_cloud_usage_probe_schedule_rejected", "account_id", account.ID)
	}
}

func (service *RateLimitService) applyOllamaCloudUsageProbeReset(
	accountID int64,
	expectedFingerprint string,
	expectedLimitedAt, expectedResetAt *time.Time,
	resetAt time.Time,
) {
	now := time.Now()
	if service == nil || accountID <= 0 || service.accountRepo == nil || !resetAt.After(now) {
		return
	}
	if expectedResetAt != nil && !resetAt.After(*expectedResetAt) {
		return
	}

	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), ollamaCloudUsageProbeWritebackTimeout)
	defer cancel()

	account, err := service.accountRepo.GetByID(bgCtx, accountID)
	if err != nil || account == nil {
		slog.Warn("ollama_cloud_usage_probe_reset_load_failed", "account_id", accountID, "error", err)
		return
	}
	if !IsOllamaCloudUsageAccount(account) {
		return
	}
	if !account.IsActive() || !account.Schedulable {
		return
	}
	currentFingerprint, valid := ollamaCloudUsageRateLimitFingerprint(account)
	if !valid || currentFingerprint != expectedFingerprint {
		return
	}

	setter, ok := service.accountRepo.(ollamaCloudUsageRateLimitSetterIfGeneration)
	if !ok {
		slog.Error("ollama_rate_limit_generation_persistence_unavailable", "account_id", accountID)
		return
	}
	updated, err := setter.SetRateLimitedIfUnchanged(bgCtx, accountID, account.UpdatedAt, expectedLimitedAt, expectedResetAt, resetAt)
	if err != nil {
		slog.Warn("rate_limit_set_failed", "account_id", accountID, "error", err)
		return
	}
	if !updated {
		slog.Debug("ollama_cloud_usage_probe_reset_skipped_stale", "account_id", accountID)
		return
	}

	service.notifyAccountSchedulingBlocked(account, resetAt, "ollama_cloud_usage_429_probe")
	slog.Info("ollama_cloud_account_rate_limited_probe",
		"account_id", accountID,
		"reset_at", resetAt.UTC(),
		"reset_in", time.Until(resetAt).Truncate(time.Second),
	)
}
