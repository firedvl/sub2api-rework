package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const OpenAIAutoWarmupEvaluationExtraKey = "codex_auto_warmup_evaluation"

type OpenAIAutoWarmupEvaluation struct {
	Reason         string `json:"reason"`
	CheckedAt      string `json:"checked_at"`
	ObservedAt     string `json:"observed_at,omitempty"`
	NextEligibleAt string `json:"next_eligible_at,omitempty"`
}

func warmupWaitingReason(account *Account, usage *OpenAIQuotaUsage, now time.Time) string {
	if !account.IsSchedulable() || account.RateLimitedAt != nil || account.RateLimitResetAt != nil {
		return "account_not_schedulable"
	}
	if usage == nil || usage.RateLimit == nil || !usage.RateLimit.allowedPresent || !usage.RateLimit.limitReachedPresent {
		return "quota_unavailable"
	}
	window := openAIAutoWarmupFiveHourWindow(usage.RateLimit)
	if window == nil || !window.usedPercentPresent {
		return "quota_unavailable"
	}
	if state := openAIAutoWarmupStateFromExtra(account.Extra); state != nil {
		if state.Status == OpenAIAutoWarmupStatusPending {
			return "already_attempted"
		}
		if isOpenAIAutoWarmupPreflightFailure(state) {
			attemptedAt, err := time.Parse(time.RFC3339, state.AttemptedAt)
			if err == nil && now.Before(attemptedAt.Add(OpenAIAutoWarmupPreflightRetry)) {
				return "retry_floor"
			}
		}
	}
	if window.UsedPercent <= openAIAutoWarmupIdleMaxUsed && usage.RateLimit.Allowed && !usage.RateLimit.LimitReached &&
		absOpenAIAutoWarmupDuration(time.Duration(window.ResetAfterSeconds)*time.Second-openAIAutoWarmupWindowLength) <= openAIAutoWarmupHorizonSlack {
		return "waiting_second_observation"
	}
	return "waiting_new_window"
}

func (s *OpenAIQuotaAutoResetService) persistOpenAIAutoWarmupEvaluation(ctx context.Context, account *Account, reason string, now time.Time) {
	if !ResolveOpenAIAutoWarmupEnabled(account) {
		return
	}
	evaluation := OpenAIAutoWarmupEvaluation{Reason: reason, CheckedAt: now.UTC().Format(time.RFC3339)}
	if observedAt, ok := account.Extra["codex_usage_updated_at"]; ok {
		evaluation.ObservedAt = fmt.Sprint(observedAt)
	}
	if reason != "quota_unavailable" && reason != "credential_attention" {
		evaluation.ObservedAt = evaluation.CheckedAt
	} else {
		evaluation.NextEligibleAt = now.Add(OpenAIAutoWarmupPreflightRetry).UTC().Format(time.RFC3339)
	}
	if state := openAIAutoWarmupStateFromExtra(account.Extra); reason != "quota_unavailable" && reason != "credential_attention" && state != nil && isOpenAIAutoWarmupPreflightFailure(state) {
		if attemptedAt, err := time.Parse(time.RFC3339, state.AttemptedAt); err == nil {
			evaluation.NextEligibleAt = attemptedAt.Add(OpenAIAutoWarmupPreflightRetry).UTC().Format(time.RFC3339)
		}
	} else if reason == "waiting_new_window" {
		if resetAt, err := parseTime(fmt.Sprint(account.Extra["codex_5h_reset_at"])); err == nil && now.Before(resetAt) {
			evaluation.NextEligibleAt = resetAt.UTC().Format(time.RFC3339)
		}
	}
	// Runtime status is advisory; the durable claim remains the dispatch authority.
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{OpenAIAutoWarmupEvaluationExtraKey: evaluation}); err != nil {
		slog.Warn("openai_auto_warmup_evaluation_write_failed", "account_id", account.ID)
	}
}

func openAIAutoWarmupQuotaRetryPending(extra map[string]any, now time.Time) bool {
	raw, err := json.Marshal(extra[OpenAIAutoWarmupEvaluationExtraKey])
	if err != nil {
		return false
	}
	var evaluation OpenAIAutoWarmupEvaluation
	if json.Unmarshal(raw, &evaluation) != nil || (evaluation.Reason != "quota_unavailable" && evaluation.Reason != "credential_attention") {
		return false
	}
	checkedAt, err := time.Parse(time.RFC3339, evaluation.CheckedAt)
	return err == nil && now.Sub(checkedAt) >= 0 && now.Sub(checkedAt) < OpenAIAutoWarmupPreflightRetry
}

func openAIAutoWarmupQuotaFailureReason(err error) string {
	switch infraerrors.Reason(err) {
	case "OPENAI_QUOTA_TOKEN_UNAVAILABLE", "OPENAI_QUOTA_AUTH_FAILED", "OPENAI_QUOTA_MISSING_ACCOUNT_ID":
		return "credential_attention"
	default:
		return "quota_unavailable"
	}
}
