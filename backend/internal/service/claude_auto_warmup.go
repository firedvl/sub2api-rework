package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/gin-gonic/gin"
)

const (
	ClaudeAutoWarmupStateExtraKey      = "claude_auto_warmup_state"
	ClaudeAutoWarmupEvaluationExtraKey = "claude_auto_warmup_evaluation"
	claudeAutoWarmupWindowType         = "claude_5h"
)

func IsClaudeAutoWarmupConfigurable(account *Account) bool {
	return account != nil && account.Platform == PlatformAnthropic && account.Type == AccountTypeOAuth && !account.IsShadow()
}

func IsAutoWarmupConfigurable(account *Account) bool {
	return IsOpenAIAutoWarmupConfigurable(account) || IsClaudeAutoWarmupConfigurable(account)
}

func (s *OpenAIQuotaAutoResetService) scanClaudeAutoWarmupAccounts(ctx context.Context) {
	if s.claudeWarmupTest == nil || s.claudeWarmupUsage == nil || s.claudeWarmupTokens == nil || s.warmupAttempts == nil {
		return
	}
	for page := 1; ; page++ {
		accounts, info, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{Page: page, PageSize: openAIAutoResetBatchSize}, PlatformAnthropic, AccountTypeOAuth, StatusActive, "", 0, "")
		if err != nil {
			slog.Warn("claude_auto_warmup_scan_failed", "page", page)
			return
		}
		for i := range accounts {
			a := &accounts[i]
			if IsClaudeAutoWarmupConfigurable(a) && a.IsSchedulable() && resolveAccountExtraBool(a.Extra, OpenAIAutoWarmupEnabledExtraKey) {
				s.Notify(a.ID)
			}
		}
		if len(accounts) < openAIAutoResetBatchSize || info == nil || page >= info.Pages {
			return
		}
	}
}

// A live five-hour window needs no request. Missing or malformed quota data never authorizes inference.
func claudeAutoWarmupReady(usage *ClaudeUsageResponse, now time.Time) (bool, string, time.Time) {
	if usage == nil || usage.FiveHour.Utilization == nil {
		return false, "quota_unavailable", time.Time{}
	}
	if usage.FiveHour.ResetsAt != "" {
		reset, err := parseTime(usage.FiveHour.ResetsAt)
		if err != nil || reset.After(now.Add(6*time.Hour)) {
			return false, "quota_unavailable", time.Time{}
		}
	}
	for _, window := range []ClaudeUsageWindow{usage.FiveHour, usage.SevenDay, usage.SevenDaySonnet, usage.SevenDayOverageIncluded} {
		if window.Utilization == nil {
			continue
		}
		used := *window.Utilization
		if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used > 100 {
			return false, "quota_unavailable", time.Time{}
		}
		if window.ResetsAt != "" {
			reset, err := parseTime(window.ResetsAt)
			if err != nil {
				return false, "quota_unavailable", time.Time{}
			}
			if now.Before(reset) && used >= 100 {
				return false, "waiting_new_window", reset
			}
		} else if used > 0 {
			return false, "quota_unavailable", time.Time{}
		}
	}
	if usage.FiveHour.ResetsAt != "" {
		reset, err := parseTime(usage.FiveHour.ResetsAt)
		if err != nil {
			return false, "quota_unavailable", time.Time{}
		}
		if now.Before(reset) {
			return false, "waiting_new_window", reset
		}
	}
	return true, "", time.Time{}
}

func (s *OpenAIQuotaAutoResetService) persistClaudeWarmupEvaluation(ctx context.Context, a *Account, reason string, next time.Time) error {
	evaluation := OpenAIAutoWarmupEvaluation{Reason: reason, CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if !next.IsZero() {
		evaluation.NextEligibleAt = next.UTC().Format(time.RFC3339)
	}
	return s.accountRepo.UpdateExtra(ctx, a.ID, map[string]any{ClaudeAutoWarmupEvaluationExtraKey: evaluation})
}

// Publish the same observations used by the account usage cells and countdowns.
func (s *OpenAIQuotaAutoResetService) fetchClaudeWarmupUsage(ctx context.Context, a *Account, token string) (*ClaudeUsageResponse, error) {
	if a.ProxyID != nil && a.Proxy == nil {
		return nil, fmt.Errorf("Claude warm-up proxy unavailable")
	}
	account := *a
	account.Credentials = cloneOpenAIAutoResetExtra(a.Credentials)
	if account.Credentials == nil {
		account.Credentials = make(map[string]any)
	}
	account.Credentials["access_token"] = token
	observed := time.Now().UTC()
	usage, err := s.claudeWarmupUsage.fetchOAuthUsageRaw(ctx, &account)
	if err != nil {
		return nil, err
	}
	if err := s.claudeWarmupUsage.syncActiveToPassive(ctx, a.ID, s.claudeWarmupUsage.buildUsageInfo(usage, &observed)); err != nil {
		return nil, err
	}
	if s.claudeWarmupUsage.cache != nil {
		s.claudeWarmupUsage.cache.apiCache.Store(a.ID, &apiUsageCache{response: usage, timestamp: observed})
	}
	return usage, nil
}

func claudeAutoWarmupStateFromExtra(extra map[string]any) *OpenAIAutoWarmupState {
	raw, _ := json.Marshal(extra[ClaudeAutoWarmupStateExtraKey])
	var state OpenAIAutoWarmupState
	if json.Unmarshal(raw, &state) != nil || state.Status == "" {
		return nil
	}
	return &state
}

func claudeAutoWarmupUsageReset(usage *ClaudeUsageResponse, now time.Time) (time.Time, bool) {
	if usage == nil || usage.FiveHour.Utilization == nil {
		return time.Time{}, false
	}
	used := *usage.FiveHour.Utilization
	if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used > 100 {
		return time.Time{}, false
	}
	reset, err := parseTime(usage.FiveHour.ResetsAt)
	return reset, err == nil && reset.After(now) && !reset.After(now.Add(6*time.Hour))
}

func (s *OpenAIQuotaAutoResetService) confirmClaudeWarmupWindow(ctx context.Context, a *Account, state *OpenAIAutoWarmupState, reset time.Time) error {
	start := reset.Add(-5 * time.Hour)
	// Use the provider's reset, never a locally invented five-hour countdown.
	status := "allowed"
	if state.Observed5hUsedPercent != nil && *state.Observed5hUsedPercent >= 100 {
		status = "rejected"
	}
	if err := s.accountRepo.UpdateSessionWindow(ctx, a.ID, &start, &reset, status); err != nil {
		return err
	}
	state.WindowStarted = true
	state.Observed5hResetAt = reset.UTC().Format(time.RFC3339)
	if state.Status != OpenAIAutoWarmupStatusSucceeded {
		state.Status = OpenAIAutoWarmupStatusWindowStartedWithWarning
	}
	if err := s.accountRepo.UpdateExtra(ctx, a.ID, map[string]any{ClaudeAutoWarmupStateExtraKey: state}); err != nil {
		return err
	}
	return s.persistClaudeWarmupEvaluation(ctx, a, "current_window_complete", reset)
}

func (s *OpenAIQuotaAutoResetService) evaluateClaudeAutoWarmup(ctx context.Context, a *Account) error {
	if !IsClaudeAutoWarmupConfigurable(a) || !a.IsSchedulable() || a.IsSyntheticUITest() || !resolveAccountExtraBool(a.Extra, OpenAIAutoWarmupEnabledExtraKey) ||
		s.claudeWarmupTest == nil || s.claudeWarmupUsage == nil || s.claudeWarmupTokens == nil || s.warmupAttempts == nil {
		return nil
	}
	now := time.Now()
	state := claudeAutoWarmupStateFromExtra(a.Extra)
	var evaluation OpenAIAutoWarmupEvaluation
	raw, _ := json.Marshal(a.Extra[ClaudeAutoWarmupEvaluationExtraKey])
	if json.Unmarshal(raw, &evaluation) == nil {
		if next, err := parseTime(evaluation.NextEligibleAt); err == nil && now.Before(next) {
			return nil
		}
	}
	retryAt := now.Add(OpenAIAutoWarmupPreflightRetry)
	token, err := s.claudeWarmupTokens.GetAccessToken(ctx, a)
	if err != nil || strings.TrimSpace(token) == "" {
		return s.persistClaudeWarmupEvaluation(ctx, a, "credential_attention", retryAt)
	}
	usage, err := s.fetchClaudeWarmupUsage(ctx, a, token)
	if err != nil {
		var limited *ClaudeUsageRateLimitError
		if errors.As(err, &limited) && limited.RetryAt.After(retryAt) {
			retryAt = limited.RetryAt
		}
		return s.persistClaudeWarmupEvaluation(ctx, a, "quota_unavailable", retryAt)
	}
	attemptedAt := time.Time{}
	if state != nil {
		attemptedAt, _ = parseTime(state.AttemptedAt)
	}
	if state != nil && !state.WindowStarted && !attemptedAt.IsZero() && now.Sub(attemptedAt) >= 0 && now.Sub(attemptedAt) < 6*time.Hour {
		if reset, ok := claudeAutoWarmupUsageReset(usage, time.Now()); ok {
			state.Observed5hUsedPercent = usage.FiveHour.Utilization
			return s.confirmClaudeWarmupWindow(ctx, a, state, reset)
		}
	}
	ready, reason, next := claudeAutoWarmupReady(usage, now)
	if !ready {
		if next.IsZero() {
			next = retryAt
		}
		return s.persistClaudeWarmupEvaluation(ctx, a, reason, next)
	}
	// A fresh idle observation plus the durable retry floor prevents duplicate or rapid sends.
	attempt, claimed, err := s.warmupAttempts.ClaimDormant(ctx, a.ID, claudeAutoWarmupWindowType, now.Add(5*time.Hour), OpenAIAutoWarmupPreflightRetry)
	if err != nil {
		return s.persistClaudeWarmupEvaluation(ctx, a, "claim_unavailable", retryAt)
	}
	if !claimed || attempt == nil {
		if attempt != nil {
			retryAt = attempt.AttemptedAt.Add(OpenAIAutoWarmupPreflightRetry)
			if check := now.Add(time.Minute); check.Before(retryAt) {
				retryAt = check
			}
		}
		return s.persistClaudeWarmupEvaluation(ctx, a, "retry_floor", retryAt)
	}
	state = &OpenAIAutoWarmupState{Status: OpenAIAutoWarmupStatusPending, WindowType: claudeAutoWarmupWindowType,
		ResetAt: attempt.ResetAt.UTC().Format(time.RFC3339), AttemptedAt: attempt.AttemptedAt.UTC().Format(time.RFC3339), Model: claude.DefaultTestModel}
	if err := s.accountRepo.UpdateExtra(ctx, a.ID, map[string]any{ClaudeAutoWarmupStateExtraKey: state}); err != nil {
		return err
	}
	started := time.Now()
	var responseHeader http.Header
	upstreamStatus := 0
	// Re-read the opt-in immediately before dispatch, as the queue and quota lookup may take time.
	current, sendErr := s.accountRepo.GetByID(ctx, a.ID)
	if sendErr == nil && (!IsClaudeAutoWarmupConfigurable(current) || !current.IsSchedulable() || current.IsSyntheticUITest() || !resolveAccountExtraBool(current.Extra, OpenAIAutoWarmupEnabledExtraKey)) {
		sendErr = fmt.Errorf("Claude warm-up disabled or account unavailable")
	}
	if sendErr == nil && current.ProxyID != nil && current.Proxy == nil {
		sendErr = fmt.Errorf("Claude warm-up proxy unavailable")
	}
	if sendErr == nil {
		token, sendErr = s.claudeWarmupTokens.GetAccessToken(ctx, current)
		if sendErr == nil && strings.TrimSpace(token) == "" {
			sendErr = fmt.Errorf("Claude OAuth token unavailable")
		}
	}
	if sendErr == nil {
		// Use the connection tester's request, headers, TLS/proxy handling and native stream completion checks.
		copy := *current
		copy.Credentials = cloneOpenAIAutoResetExtra(current.Credentials)
		if copy.Credentials == nil {
			copy.Credentials = make(map[string]any)
		}
		copy.Credentials["access_token"] = token
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = (&http.Request{}).WithContext(ctx)
		requestCtx, cancel := context.WithTimeout(ctx, openAIAutoWarmupRequestTimeout)
		c.Request = c.Request.WithContext(requestCtx)
		sendErr = s.claudeWarmupTest.testClaudeConnection(c, &copy, state.Model, 16, func(resp *http.Response) {
			responseHeader, upstreamStatus = resp.Header.Clone(), resp.StatusCode
		})
		cancel()
	}
	state.UpstreamStatus = upstreamStatus
	state.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	state.Status = OpenAIAutoWarmupStatusSucceeded
	completion := OpenAIAutoWarmupCompletion{Status: state.Status, Model: state.Model, LatencyMS: int(time.Since(started).Milliseconds())}
	if sendErr != nil {
		state.Status = OpenAIAutoWarmupStatusFailed
		state.ErrorCode = "CLAUDE_AUTO_WARMUP_FAILED"
		completion.Status = state.Status
		completion.ErrorCode = state.ErrorCode
	}
	completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := s.warmupAttempts.Complete(completeCtx, attempt.ID, completion); err != nil {
		slog.Warn("claude_auto_warmup_complete_failed", "account_id", a.ID)
	}
	if err := s.accountRepo.UpdateExtra(completeCtx, a.ID, map[string]any{ClaudeAutoWarmupStateExtraKey: state}); err != nil {
		return err
	}
	slog.Info("claude_auto_warmup_completed", "account_id", a.ID, "status", state.Status)
	// Headers can confirm a window even if the body was interrupted afterwards.
	if reset, ok := parseAnthropicWindowReset(responseHeader, "5h", time.Now()); ok &&
		(upstreamStatus == http.StatusOK || strings.HasPrefix(responseHeader.Get("anthropic-ratelimit-unified-5h-status"), "allowed")) {
		updates := map[string]any{"session_window_usage_reset": reset.Unix()}
		if used, ok := claudeUsageNumber(responseHeader.Get("anthropic-ratelimit-unified-5h-utilization")); ok {
			percent := used * 100
			state.Observed5hUsedPercent = &percent
			updates["session_window_utilization"] = used
			updates["session_window_utilization_source"] = "passive"
			updates["session_window_utilization_observed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if err := s.accountRepo.UpdateExtra(completeCtx, a.ID, updates); err != nil {
			return err
		}
		if s.claudeWarmupUsage.cache != nil {
			s.claudeWarmupUsage.cache.apiCache.Delete(a.ID)
		}
		return s.confirmClaudeWarmupWindow(completeCtx, a, state, reset)
	}
	// A completed stream alone is insufficient: read back the actual window.
	verificationRetry := time.Now().Add(time.Minute)
	if current != nil && token != "" {
		verifyCtx, cancel := context.WithTimeout(completeCtx, 10*time.Second)
		observed, err := s.fetchClaudeWarmupUsage(verifyCtx, current, token)
		cancel()
		var limited *ClaudeUsageRateLimitError
		if errors.As(err, &limited) && limited.RetryAt.After(verificationRetry) {
			verificationRetry = limited.RetryAt
		}
		if err == nil {
			if reset, ok := claudeAutoWarmupUsageReset(observed, time.Now()); ok {
				state.Observed5hUsedPercent = observed.FiveHour.Utilization
				return s.confirmClaudeWarmupWindow(completeCtx, a, state, reset)
			}
		}
	}
	reason = "request_completed_reset_unverified"
	if sendErr != nil {
		reason = "retry_floor"
	}
	return s.persistClaudeWarmupEvaluation(completeCtx, a, reason, verificationRetry)
}
