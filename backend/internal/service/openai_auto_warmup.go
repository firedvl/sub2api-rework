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
	OpenAIAutoWarmupStatusPending   = "pending"
	OpenAIAutoWarmupStatusSucceeded = "succeeded"
	OpenAIAutoWarmupStatusFailed    = "failed"

	openAIAutoWarmupWindowType   = "5h"
	openAIAutoWarmupResetAdvance = time.Minute
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
	Complete(ctx context.Context, attemptID int64, completion OpenAIAutoWarmupCompletion) error
}

type OpenAIAutoWarmupResult struct {
	Model      string
	RequestID  string
	ResponseID string
	Usage      OpenAIUsage
	Latency    time.Duration
}

type openAIAutoWarmupSender interface {
	SendOpenAIAutoWarmup(ctx context.Context, accountID int64) (*OpenAIAutoWarmupResult, error)
}

type OpenAIAutoWarmupState struct {
	Status      string `json:"status"`
	WindowType  string `json:"window_type"`
	ResetAt     string `json:"reset_at"`
	AttemptedAt string `json:"attempted_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	ErrorCode   string `json:"error_code,omitempty"`
	Model       string `json:"model,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
}

type openAIAutoWarmupWindow struct {
	resetAt time.Time
}

func assessOpenAIAutoWarmupWindow(account *Account, usage *OpenAIQuotaUsage, recovered bool, now time.Time) (openAIAutoWarmupWindow, bool) {
	if account == nil || usage == nil || usage.RateLimit == nil ||
		!usage.RateLimit.allowedPresent || !usage.RateLimit.limitReachedPresent ||
		!usage.RateLimit.Allowed || usage.RateLimit.LimitReached {
		return openAIAutoWarmupWindow{}, false
	}
	updates := buildOpenAIAutoResetUsageUpdates(usage, now)
	newReset, err := parseTime(strings.TrimSpace(fmt.Sprint(updates["codex_5h_reset_at"])))
	if err != nil || newReset.IsZero() {
		return openAIAutoWarmupWindow{}, false
	}
	newWindowMinutes := int(parseExtraFloat64(updates["codex_5h_window_minutes"]))
	oldWindowMinutes := int(parseExtraFloat64(account.Extra["codex_5h_window_minutes"]))
	if newWindowMinutes <= 0 || oldWindowMinutes <= 0 || newWindowMinutes != oldWindowMinutes || newWindowMinutes > 360 {
		return openAIAutoWarmupWindow{}, false
	}
	oldReset, err := parseTime(strings.TrimSpace(fmt.Sprint(account.Extra["codex_5h_reset_at"])))
	if err != nil || oldReset.IsZero() || newReset.Sub(oldReset) < openAIAutoWarmupResetAdvance {
		return openAIAutoWarmupWindow{}, false
	}
	used := parseExtraFloat64(updates["codex_5h_used_percent"])
	if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used >= 100 {
		return openAIAutoWarmupWindow{}, false
	}
	if !recovered && now.Before(oldReset) {
		return openAIAutoWarmupWindow{}, false
	}
	return openAIAutoWarmupWindow{resetAt: newReset.UTC()}, true
}

func (s *OpenAIQuotaAutoResetService) maybeWarmFreshOpenAIWindow(ctx context.Context, previous *Account, usage *OpenAIQuotaUsage, recovered bool, now time.Time) {
	if s == nil || s.accountRepo == nil || s.warmupAttempts == nil || s.warmupSender == nil || previous == nil || !ResolveOpenAIAutoWarmupEnabled(previous) || !s.openAIAutoWarmupEnabled(ctx) {
		return
	}
	window, ok := assessOpenAIAutoWarmupWindow(previous, usage, recovered, now)
	if !ok {
		return
	}
	attempt, claimed, err := s.warmupAttempts.Claim(ctx, previous.ID, openAIAutoWarmupWindowType, window.resetAt)
	if err != nil {
		slog.Warn("openai_auto_warmup_claim_failed", "account_id", previous.ID, "error_code", infraerrors.Reason(err))
		return
	}
	if !claimed || attempt == nil {
		return
	}
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
	}
	if sendErr != nil {
		completion.Status = OpenAIAutoWarmupStatusFailed
		completion.ErrorCode = firstNonEmpty(infraerrors.Reason(sendErr), "OPENAI_AUTO_WARMUP_FAILED")
		state.Status = OpenAIAutoWarmupStatusFailed
		state.ErrorCode = completion.ErrorCode
	}
	completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.warmupAttempts.Complete(completeCtx, attempt.ID, completion); err != nil {
		slog.Warn("openai_auto_warmup_complete_failed", "account_id", accountID, "attempt_id", attempt.ID, "error_code", infraerrors.Reason(err))
	}
	s.persistOpenAIAutoWarmupState(completeCtx, accountID, state)
	if sendErr != nil {
		slog.Warn("openai_auto_warmup_failed", "account_id", accountID, "attempt_id", attempt.ID, "error_code", completion.ErrorCode)
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
