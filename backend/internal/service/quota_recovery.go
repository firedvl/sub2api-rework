package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	QuotaRateLimitBlockExtraKey       = "quota_rate_limit_block"
	quotaRateLimitReasonExhausted     = "quota_exhausted"
	openAIQuotaRecoveryMinimumAge     = 30 * time.Second
	openAIQuotaRecoveryMaxUsedPercent = 95.0
)

// QuotaRateLimitBlock identifies the exact quota-originated cooldown generation
// that fresh provider evidence may recover. Generic 429 and Retry-After blocks
// deliberately do not receive this marker.
type QuotaRateLimitBlock struct {
	Provider            string             `json:"provider"`
	Reason              string             `json:"reason"`
	Source              string             `json:"source"`
	LimitingWindows     []string           `json:"limiting_windows"`
	ObservedUtilization map[string]float64 `json:"observed_utilization,omitempty"`
	BlockedAt           time.Time          `json:"blocked_at"`
	ResetAt             time.Time          `json:"reset_at"`
}

type quotaRateLimitBlockRepository interface {
	SetQuotaRateLimited(ctx context.Context, id int64, resetAt time.Time, block QuotaRateLimitBlock) (bool, error)
	ClearQuotaRateLimitIfObserved(ctx context.Context, id int64, block QuotaRateLimitBlock) (bool, error)
}

type quotaRecoveryRuntimeBlockClearer interface {
	ClearQuotaRecoveryRuntimeBlock(accountID int64)
}

type quotaAccountRuntimeBlocker interface {
	BlockQuotaAccountScheduling(account *Account, until time.Time) uint64
	ClearQuotaAccountSchedulingBlock(accountID int64, generation uint64)
	ClearQuotaRecoveryRuntimeBlock(accountID int64)
}

type openAIQuotaRecoveryDecision struct {
	FreshRemaining map[string]float64
	FreshResetAt   map[string]int64
}

func newOpenAIQuotaRateLimitBlock(source string, utilization map[string]float64) QuotaRateLimitBlock {
	windows := make([]string, 0, len(utilization))
	for window := range utilization {
		windows = append(windows, window)
	}
	sort.Strings(windows)
	return QuotaRateLimitBlock{
		Provider:            PlatformOpenAI,
		Reason:              quotaRateLimitReasonExhausted,
		Source:              source,
		LimitingWindows:     windows,
		ObservedUtilization: utilization,
	}
}

func quotaRateLimitBlockFromAccount(account *Account) (*QuotaRateLimitBlock, bool) {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth || account.IsShadow() ||
		account.Status != StatusActive || !account.Schedulable || account.RateLimitedAt == nil || account.RateLimitResetAt == nil {
		return nil, false
	}
	raw, ok := account.Extra[QuotaRateLimitBlockExtraKey]
	if !ok || raw == nil {
		return nil, false
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var block QuotaRateLimitBlock
	if err := json.Unmarshal(encoded, &block); err != nil {
		return nil, false
	}
	if block.Provider != PlatformOpenAI || block.Reason != quotaRateLimitReasonExhausted || len(block.LimitingWindows) == 0 ||
		block.BlockedAt.IsZero() || block.ResetAt.IsZero() ||
		!block.BlockedAt.Equal(*account.RateLimitedAt) || !block.ResetAt.Equal(*account.RateLimitResetAt) {
		return nil, false
	}
	return &block, true
}

func assessOpenAIQuotaRecovery(account *Account, usage *OpenAIQuotaUsage, now time.Time) (*QuotaRateLimitBlock, *openAIQuotaRecoveryDecision, bool) {
	block, ok := quotaRateLimitBlockFromAccount(account)
	if !ok || now.Sub(block.BlockedAt) < openAIQuotaRecoveryMinimumAge || usage == nil || usage.RateLimit == nil ||
		!usage.RateLimit.allowedPresent || !usage.RateLimit.limitReachedPresent ||
		!usage.RateLimit.Allowed || usage.RateLimit.LimitReached || !openAIQuotaWindowsAreComplete(usage.RateLimit) {
		return nil, nil, false
	}

	updates := buildOpenAIAutoResetUsageUpdates(usage, now)
	used := make(map[string]float64, 2)
	resets := make(map[string]int64, 2)
	for _, window := range []string{"5h", "7d"} {
		if value, exists := updates["codex_"+window+"_used_percent"]; exists {
			used[window] = parseExtraFloat64(value)
		}
		if value, exists := updates["codex_"+window+"_reset_at"]; exists {
			if resetAt, err := parseTime(strings.TrimSpace(fmt.Sprint(value))); err == nil {
				resets[window] = resetAt.Unix()
			}
		}
	}
	if len(used) == 0 {
		return nil, nil, false
	}
	for _, percent := range used {
		if percent > openAIQuotaRecoveryMaxUsedPercent {
			return nil, nil, false
		}
	}
	for _, window := range block.LimitingWindows {
		if window == "global" {
			continue
		}
		if _, exists := used[window]; !exists {
			return nil, nil, false
		}
	}

	remaining := make(map[string]float64, len(used))
	for window, percent := range used {
		remaining[window] = 100 - percent
	}
	return block, &openAIQuotaRecoveryDecision{FreshRemaining: remaining, FreshResetAt: resets}, true
}

func openAIQuotaWindowsAreComplete(rateLimit *OpenAIRateLimit) bool {
	known := false
	for _, window := range []*OpenAIRateLimitWindow{rateLimit.PrimaryWindow, rateLimit.SecondaryWindow} {
		if window == nil {
			continue
		}
		known = true
		if !window.usedPercentPresent || window.LimitWindowSeconds <= 0 ||
			math.IsNaN(window.UsedPercent) || math.IsInf(window.UsedPercent, 0) ||
			window.UsedPercent < 0 || window.UsedPercent > 100 {
			return false
		}
	}
	return known
}

func (s *OpenAIQuotaAutoResetService) tryRecoverQuotaBlock(ctx context.Context, account *Account, usage *OpenAIQuotaUsage, now time.Time) (bool, error) {
	block, decision, ok := assessOpenAIQuotaRecovery(account, usage, now)
	if !ok {
		return false, nil
	}
	repo, ok := s.accountRepo.(quotaRateLimitBlockRepository)
	if !ok {
		return false, nil
	}
	recovered, err := repo.ClearQuotaRateLimitIfObserved(ctx, account.ID, *block)
	if err != nil || !recovered {
		return false, err
	}
	if clearer, ok := s.recoverer.(quotaRecoveryRuntimeBlockClearer); ok {
		clearer.ClearQuotaRecoveryRuntimeBlock(account.ID)
	}
	recoveredAt := time.Now().UTC()
	slog.Info("account_quota_recovered",
		"account_id", account.ID,
		"provider", block.Provider,
		"previous_status", account.Status,
		"block_reason", block.Reason,
		"blocked_at", block.BlockedAt,
		"old_reset_at", block.ResetAt,
		"recovery_evidence", "fresh_provider_quota",
		"fresh_remaining_percent", decision.FreshRemaining,
		"limiting_windows", block.LimitingWindows,
		"new_reset_at", decision.FreshResetAt,
		"recovered_at", recoveredAt,
	)
	return true, nil
}

func (s *RateLimitService) setOpenAIQuotaRateLimited(ctx context.Context, account *Account, resetAt time.Time, block QuotaRateLimitBlock, runtimeGeneration uint64) error {
	repo, ok := s.accountRepo.(quotaRateLimitBlockRepository)
	if !ok {
		return s.accountRepo.SetRateLimited(ctx, account.ID, resetAt)
	}
	applied, err := repo.SetQuotaRateLimited(ctx, account.ID, resetAt, block)
	if err != nil {
		return err
	}
	if blocker, ok := s.runtimeBlocker.(quotaAccountRuntimeBlocker); ok {
		blocker.ClearQuotaAccountSchedulingBlock(account.ID, runtimeGeneration)
	}
	if !applied {
		return nil
	}
	notifyOpenAIQuotaRecovery(account.ID)
	return nil
}

func (s *RateLimitService) ClearQuotaRecoveryRuntimeBlock(accountID int64) {
	if blocker, ok := s.runtimeBlocker.(quotaAccountRuntimeBlocker); ok {
		blocker.ClearQuotaRecoveryRuntimeBlock(accountID)
	}
}

func (s *RateLimitService) notifyQuotaAccountSchedulingBlocked(account *Account, until time.Time) uint64 {
	if blocker, ok := s.runtimeBlocker.(quotaAccountRuntimeBlocker); ok {
		return blocker.BlockQuotaAccountScheduling(account, until)
	}
	s.notifyAccountSchedulingBlocked(account, until, "429")
	return 0
}
