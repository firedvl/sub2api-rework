package service

import (
	"context"
	"time"
)

// UsageExhaustion describes a pool in which every account that could serve the
// requested model is blocked until a known time: provider rate limit, quota
// window, or cooldown. Handlers use it on the "no available accounts" path to
// tell the client when usage comes back.
type UsageExhaustion struct {
	// ResetAt is the earliest time any candidate account becomes usable again.
	ResetAt time.Time
	// Window is the provider usage window ("5h" or "7d") whose reset ResetAt
	// is, when the binding account's stored window state identifies it. It is
	// empty when the block is not a recognized usage window.
	Window string
}

// usageExhaustionScan accumulates the usage-exhaustion verdict over the
// model-capable accounts of a pool.
type usageExhaustionScan struct {
	now      time.Time
	usable   bool
	earliest UsageExhaustion
}

// observe records one account that is configured to serve requestedModel.
func (s *usageExhaustionScan) observe(ctx context.Context, account *Account, requestedModel string) {
	until, blocked := account.usageBlockedUntil(ctx, requestedModel, s.now)
	if !blocked {
		// The account is schedulable right now, so the selection failure has
		// another cause (concurrency, RPM, profit control, ...).
		s.usable = true
		return
	}
	if until.IsZero() {
		// Blocked without a known reset: it does not contribute a reset time.
		return
	}
	if s.earliest.ResetAt.IsZero() || until.Before(s.earliest.ResetAt) {
		s.earliest = UsageExhaustion{ResetAt: until, Window: account.usageWindowResettingAt(until, s.now)}
	}
}

// result returns nil unless every observed account is blocked and at least one
// of them has a known future reset.
func (s *usageExhaustionScan) result() *UsageExhaustion {
	if s.usable || s.earliest.ResetAt.IsZero() {
		return nil
	}
	exhaustion := s.earliest
	return &exhaustion
}

// usageBlockedUntil reports whether the account is currently blocked from
// serving requestedModel and, when every active block is time-bound, the time
// the last of them ends. blocked with a zero time means the account is blocked
// by a condition that has no known reset (expired, quota exceeded, stale quota
// snapshot).
func (a *Account) usageBlockedUntil(ctx context.Context, requestedModel string, now time.Time) (until time.Time, blocked bool) {
	if a == nil {
		return time.Time{}, true
	}
	resets := []*time.Time{a.RateLimitResetAt, a.OverloadUntil, a.TempUnschedulableUntil}
	// OpenAI quota auto-pause is evaluated by the scheduler from the stored
	// Codex snapshot and is not persisted as a cooldown.
	paused, decision := evaluateOpenAIAccountQuotaPause(ctx, a)
	if !paused && a.IsSchedulableForModelWithContext(ctx, requestedModel) {
		return time.Time{}, false
	}
	if paused {
		candidate := openAIThresholdCandidate(a.Extra, decision.window, now)
		if candidate == nil || candidate.until == nil || !candidate.until.After(now) {
			return time.Time{}, true
		}
		resets = append(resets, candidate.until)
	}

	// Blocks that remain once the time-bound cooldowns are lifted have no reset.
	unblocked := *a
	unblocked.RateLimitResetAt, unblocked.OverloadUntil, unblocked.TempUnschedulableUntil = nil, nil, nil
	if !unblocked.IsSchedulable() {
		return time.Time{}, true
	}

	for _, key := range a.modelRateLimitKeysForRequest(ctx, requestedModel) {
		resets = append(resets, a.modelRateLimitResetAt(key))
	}
	for _, reset := range resets {
		if reset != nil && reset.After(now) && reset.After(until) {
			until = *reset
		}
	}
	return until, true
}

// usageWindowResettingAt names the Anthropic usage window whose stored reset is
// exactly until. Both the upstream 429 handling and the scheduling threshold
// derive their cooldown from these stored resets, so an exact match identifies
// the binding window without guessing.
func (a *Account) usageWindowResettingAt(until, now time.Time) string {
	if a == nil || a.Platform != PlatformAnthropic {
		return ""
	}
	matches := func(reset *time.Time) bool {
		return reset != nil && reset.Unix() == until.Unix()
	}
	if matches(parseSchedulingResetAt(a.Extra["passive_usage_7d_reset"])) {
		return "7d"
	}
	// A 429 without a 5h reset header stores its reset as the session window
	// end, so only a reset within five hours can be the five-hour window.
	if until.Sub(now) <= 5*time.Hour &&
		(matches(a.SessionWindowEnd) || matches(parseSchedulingResetAt(a.Extra["session_window_usage_reset"]))) {
		return "5h"
	}
	return ""
}
