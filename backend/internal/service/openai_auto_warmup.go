package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OpenAIAutoWarmupStatusPending                  = "pending"
	OpenAIAutoWarmupStatusSucceeded                = "succeeded"
	OpenAIAutoWarmupStatusFailed                   = "failed"
	OpenAIAutoWarmupStatusWindowStartedWithWarning = "window_started_with_warning"

	openAIAutoWarmupWindowType   = "5h"
	openAIAutoWarmupResetAdvance = time.Minute
	openAIAutoWarmupWindowLength = 5 * time.Hour
	openAIAutoWarmupMinGap       = 30 * time.Second
	openAIAutoWarmupMaxGap       = 2 * openAIAutoResetSnapshotTTL
	openAIAutoWarmupHorizonSlack = 2 * time.Minute
	openAIAutoWarmupAdvanceSlack = 15 * time.Second
	openAIAutoWarmupIdleMaxUsed  = 0.1
	openAIAutoWarmupDormantRetry = 5 * time.Hour
)

type OpenAIAutoWarmupAttempt struct {
	ID                  int64
	AccountID           int64
	WindowType          string
	ResetAt             time.Time
	AttemptedAt         time.Time
	CompletedAt         *time.Time
	Status              string
	ErrorCode           string
	Model               string
	RequestID           string
	ResponseID          string
	LatencyMS           *int
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

type OpenAIAutoWarmupCompletion struct {
	Status              string
	ErrorCode           string
	Model               string
	RequestID           string
	ResponseID          string
	LatencyMS           int
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

type OpenAIAutoWarmupAttemptRepository interface {
	Claim(ctx context.Context, accountID int64, windowType string, resetAt time.Time) (*OpenAIAutoWarmupAttempt, bool, error)
	ClaimDormant(ctx context.Context, accountID int64, windowType string, resetAt time.Time, retryAfter time.Duration) (*OpenAIAutoWarmupAttempt, bool, error)
	Complete(ctx context.Context, attemptID int64, completion OpenAIAutoWarmupCompletion) error
}

type OpenAIAutoWarmupResult struct {
	Model                 string
	RequestID             string
	ResponseID            string
	Usage                 OpenAIUsage
	Latency               time.Duration
	UpstreamStatus        int
	UpstreamErrorCode     string
	UpstreamErrorMessage  string
	WindowStarted         bool
	Observed5hResetAt     string
	Observed5hUsedPercent *float64
}

type openAIAutoWarmupSender interface {
	SendOpenAIAutoWarmup(ctx context.Context, accountID int64) (*OpenAIAutoWarmupResult, error)
}

type OpenAIAutoWarmupState struct {
	Status                string   `json:"status"`
	WindowType            string   `json:"window_type"`
	ResetAt               string   `json:"reset_at"`
	AttemptedAt           string   `json:"attempted_at"`
	CompletedAt           string   `json:"completed_at,omitempty"`
	ErrorCode             string   `json:"error_code,omitempty"`
	Model                 string   `json:"model,omitempty"`
	RequestID             string   `json:"request_id,omitempty"`
	UpstreamStatus        int      `json:"upstream_status,omitempty"`
	UpstreamErrorCode     string   `json:"upstream_error_code,omitempty"`
	WindowStarted         bool     `json:"window_started,omitempty"`
	Observed5hResetAt     string   `json:"observed_5h_reset_at,omitempty"`
	Observed5hUsedPercent *float64 `json:"observed_5h_used_percent,omitempty"`
}

type openAIAutoWarmupWindow struct {
	resetAt time.Time
	dormant bool
}

func assessOpenAIAutoWarmupWindow(account *Account, usage *OpenAIQuotaUsage, recovered bool, now time.Time) (openAIAutoWarmupWindow, bool) {
	if !IsOpenAIAutoWarmupConfigurable(account) || !account.IsActive() || !account.Schedulable || usage == nil || usage.RateLimit == nil ||
		!usage.RateLimit.allowedPresent || !usage.RateLimit.limitReachedPresent ||
		!usage.RateLimit.Allowed || usage.RateLimit.LimitReached {
		return openAIAutoWarmupWindow{}, false
	}
	fiveHourWindow := openAIAutoWarmupFiveHourWindow(usage.RateLimit)
	if fiveHourWindow == nil || !fiveHourWindow.usedPercentPresent {
		return openAIAutoWarmupWindow{}, false
	}
	updates := buildOpenAIAutoResetUsageUpdates(usage, now)
	newReset, err := parseTime(strings.TrimSpace(fmt.Sprint(updates["codex_5h_reset_at"])))
	if err != nil || newReset.IsZero() {
		return openAIAutoWarmupWindow{}, false
	}
	newWindowMinutes := int(parseExtraFloat64(updates["codex_5h_window_minutes"]))
	oldWindowMinutes := int(parseExtraFloat64(account.Extra["codex_5h_window_minutes"]))
	if newWindowMinutes != int(openAIAutoWarmupWindowLength/time.Minute) || oldWindowMinutes != newWindowMinutes {
		return openAIAutoWarmupWindow{}, false
	}
	oldReset, err := parseTime(strings.TrimSpace(fmt.Sprint(account.Extra["codex_5h_reset_at"])))
	if err != nil || oldReset.IsZero() {
		return openAIAutoWarmupWindow{}, false
	}
	used := parseExtraFloat64(updates["codex_5h_used_percent"])
	if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used >= 100 {
		return openAIAutoWarmupWindow{}, false
	}
	if recovered || !now.Before(oldReset) {
		if newReset.Sub(oldReset) < openAIAutoWarmupResetAdvance {
			return openAIAutoWarmupWindow{}, false
		}
		return openAIAutoWarmupWindow{resetAt: newReset.UTC()}, true
	}
	if !isDormantOpenAIAutoWarmupWindow(account, usage, fiveHourWindow, updates, oldReset, newReset, used) {
		return openAIAutoWarmupWindow{}, false
	}
	return openAIAutoWarmupWindow{resetAt: newReset.UTC(), dormant: true}, true
}

func openAIAutoWarmupFiveHourWindow(rateLimit *OpenAIRateLimit) *OpenAIRateLimitWindow {
	if rateLimit == nil {
		return nil
	}
	for _, window := range []*OpenAIRateLimitWindow{rateLimit.PrimaryWindow, rateLimit.SecondaryWindow} {
		if window != nil && time.Duration(window.LimitWindowSeconds)*time.Second == openAIAutoWarmupWindowLength {
			return window
		}
	}
	return nil
}

func isDormantOpenAIAutoWarmupWindow(account *Account, usage *OpenAIQuotaUsage, fiveHourWindow *OpenAIRateLimitWindow, updates map[string]any, oldReset, newReset time.Time, newUsed float64) bool {
	if !account.IsSchedulable() || account.RateLimitedAt != nil || account.RateLimitResetAt != nil || newUsed > openAIAutoWarmupIdleMaxUsed ||
		usage.RateLimit.PrimaryWindow != fiveHourWindow {
		return false
	}
	oldUsed, hasOldUsed := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent")
	if !hasOldUsed || math.IsNaN(oldUsed) || math.IsInf(oldUsed, 0) || oldUsed < 0 || oldUsed > openAIAutoWarmupIdleMaxUsed {
		return false
	}
	oldObserved, err := parseTime(strings.TrimSpace(fmt.Sprint(account.Extra["codex_usage_updated_at"])))
	if err != nil || oldObserved.IsZero() {
		return false
	}
	newObserved, err := parseTime(strings.TrimSpace(fmt.Sprint(updates["codex_usage_updated_at"])))
	if err != nil || newObserved.IsZero() {
		return false
	}
	elapsed := newObserved.Sub(oldObserved)
	if elapsed < openAIAutoWarmupMinGap || elapsed > openAIAutoWarmupMaxGap {
		return false
	}
	if absOpenAIAutoWarmupDuration(oldReset.Sub(oldObserved)-openAIAutoWarmupWindowLength) > openAIAutoWarmupHorizonSlack ||
		absOpenAIAutoWarmupDuration(newReset.Sub(newObserved)-openAIAutoWarmupWindowLength) > openAIAutoWarmupHorizonSlack {
		return false
	}
	return absOpenAIAutoWarmupDuration(newReset.Sub(oldReset)-elapsed) <= openAIAutoWarmupAdvanceSlack
}

func absOpenAIAutoWarmupDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func (s *OpenAIQuotaAutoResetService) maybeWarmFreshOpenAIWindow(ctx context.Context, previous *Account, usage *OpenAIQuotaUsage, recovered bool, now time.Time) {
	if s == nil || s.accountRepo == nil || s.warmupAttempts == nil || s.warmupSender == nil || previous == nil || !ResolveOpenAIAutoWarmupEnabled(previous) || !s.openAIAutoWarmupEnabled(ctx) {
		return
	}
	window, ok := assessOpenAIAutoWarmupWindow(previous, usage, recovered, now)
	if !ok {
		return
	}
	var attempt *OpenAIAutoWarmupAttempt
	var claimed bool
	var err error
	if window.dormant {
		attempt, claimed, err = s.warmupAttempts.ClaimDormant(ctx, previous.ID, openAIAutoWarmupWindowType, window.resetAt, openAIAutoWarmupDormantRetry)
	} else {
		attempt, claimed, err = s.warmupAttempts.Claim(ctx, previous.ID, openAIAutoWarmupWindowType, window.resetAt)
	}
	if err != nil {
		slog.Warn("openai_auto_warmup_claim_failed", "account_id", previous.ID, "error_code", infraerrors.Reason(err))
		return
	}
	if !claimed || attempt == nil {
		return
	}
	if window.dormant {
		slog.Info("openai_auto_warmup_idle_confirmed", "account_id", previous.ID, "reset_at", window.resetAt)
	}
	slog.Info("openai_auto_warmup_claimed", "account_id", previous.ID, "attempt_id", attempt.ID, "dormant", window.dormant)
	s.persistOpenAIAutoWarmupState(ctx, previous.ID, &OpenAIAutoWarmupState{
		Status: OpenAIAutoWarmupStatusPending, WindowType: openAIAutoWarmupWindowType,
		ResetAt: window.resetAt.Format(time.RFC3339), AttemptedAt: attempt.AttemptedAt.UTC().Format(time.RFC3339),
	})

	select {
	case s.warmupSlots <- struct{}{}:
		defer func() { <-s.warmupSlots }()
	case <-ctx.Done():
		s.completeOpenAIAutoWarmup(ctx, previous.ID, attempt, nil, ctx.Err())
		return
	}
	if !s.openAIAutoWarmupEnabled(ctx) {
		s.completeOpenAIAutoWarmup(ctx, previous.ID, attempt, nil, infraerrors.Conflict("OPENAI_AUTO_WARMUP_DISABLED", "OpenAI Auto Warm-up is disabled"))
		return
	}
	current, err := s.accountRepo.GetByID(ctx, previous.ID)
	if err != nil {
		s.completeOpenAIAutoWarmup(ctx, previous.ID, attempt, nil, err)
		return
	}
	if err := validateOpenAIAutoWarmupAccount(current); err != nil {
		s.completeOpenAIAutoWarmup(ctx, previous.ID, attempt, nil, err)
		return
	}
	result, sendErr := s.warmupSender.SendOpenAIAutoWarmup(ctx, previous.ID)
	s.completeOpenAIAutoWarmup(ctx, previous.ID, attempt, result, sendErr)
}

func (s *OpenAIQuotaAutoResetService) openAIAutoWarmupEnabled(ctx context.Context) bool {
	if s == nil || s.settings == nil {
		return false
	}
	settings, err := s.settings.GetAllSettings(ctx)
	return err == nil && settings != nil && settings.OpenAIAutoWarmupEnabled
}

func (s *OpenAIQuotaAutoResetService) completeOpenAIAutoWarmup(ctx context.Context, accountID int64, attempt *OpenAIAutoWarmupAttempt, result *OpenAIAutoWarmupResult, sendErr error) {
	completion := OpenAIAutoWarmupCompletion{Status: OpenAIAutoWarmupStatusSucceeded}
	state := &OpenAIAutoWarmupState{
		Status: OpenAIAutoWarmupStatusSucceeded, WindowType: attempt.WindowType,
		ResetAt: attempt.ResetAt.UTC().Format(time.RFC3339), AttemptedAt: attempt.AttemptedAt.UTC().Format(time.RFC3339),
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if result != nil {
		completion.Model = result.Model
		completion.RequestID = result.RequestID
		completion.ResponseID = result.ResponseID
		completion.LatencyMS = int(result.Latency.Milliseconds())
		completion.InputTokens = result.Usage.InputTokens
		completion.OutputTokens = result.Usage.OutputTokens
		completion.CacheCreationTokens = result.Usage.CacheCreationInputTokens
		completion.CacheReadTokens = result.Usage.CacheReadInputTokens
		state.Model = result.Model
		state.RequestID = firstNonEmpty(result.RequestID, result.ResponseID)
		state.UpstreamStatus = result.UpstreamStatus
		state.UpstreamErrorCode = result.UpstreamErrorCode
		state.WindowStarted = result.WindowStarted
		state.Observed5hResetAt = result.Observed5hResetAt
		state.Observed5hUsedPercent = result.Observed5hUsedPercent
	}
	if sendErr != nil {
		completion.Status = OpenAIAutoWarmupStatusFailed
		completion.ErrorCode = firstNonEmpty(infraerrors.Reason(sendErr), "OPENAI_AUTO_WARMUP_FAILED")
		state.Status = OpenAIAutoWarmupStatusFailed
		if result != nil && result.WindowStarted {
			state.Status = OpenAIAutoWarmupStatusWindowStartedWithWarning
			completion.ErrorCode = "OPENAI_AUTO_WARMUP_WINDOW_STARTED_WARNING"
		}
		state.ErrorCode = completion.ErrorCode
	}
	completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.warmupAttempts.Complete(completeCtx, attempt.ID, completion); err != nil {
		slog.Warn("openai_auto_warmup_complete_failed", "account_id", accountID, "attempt_id", attempt.ID, "error_code", infraerrors.Reason(err))
	}
	s.persistOpenAIAutoWarmupState(completeCtx, accountID, state)
	if sendErr != nil {
		args := []any{"account_id", accountID, "attempt_id", attempt.ID, "error_code", completion.ErrorCode}
		if result != nil {
			args = append(args,
				"model", result.Model,
				"request_id", result.RequestID,
				"upstream_status", result.UpstreamStatus,
				"upstream_error_code", result.UpstreamErrorCode,
				"upstream_error", result.UpstreamErrorMessage,
			)
		}
		slog.Warn("openai_auto_warmup_failed", args...)
		return
	}
	slog.Info("openai_auto_warmup_succeeded", "account_id", accountID, "attempt_id", attempt.ID, "model", completion.Model, "input_tokens", completion.InputTokens, "output_tokens", completion.OutputTokens)
}

func (s *OpenAIQuotaAutoResetService) persistOpenAIAutoWarmupState(ctx context.Context, accountID int64, state *OpenAIAutoWarmupState) {
	if state == nil || s == nil || s.accountRepo == nil {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{OpenAIAutoWarmupStateExtraKey: state}); err != nil {
		slog.Warn("openai_auto_warmup_state_write_failed", "account_id", accountID, "error_code", infraerrors.Reason(err))
	}
}
