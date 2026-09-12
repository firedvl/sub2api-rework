package service

import (
	"context"
	"time"
)

const (
	gatewayEffectiveRuntimeAvailable              = "available"
	gatewayEffectiveRuntimeTemporarilyUnavailable = "temporarily_unavailable"
	gatewayEffectiveRuntimeUnknown                = "unknown"
)

// gatewayEffectiveOpenAIRuntimeState reads OpenAI runtime gates without
// initializing, pruning, refreshing, or notifying any runtime component.
func gatewayEffectiveOpenAIRuntimeState(ctx context.Context, svc *OpenAIGatewayService, account *Account, model string) string {
	return gatewayEffectiveOpenAIRuntimeObservation(ctx, svc, account, model, time.Now()).State
}

func gatewayEffectiveOpenAIRuntimeObservation(ctx context.Context, svc *OpenAIGatewayService, account *Account, model string, now time.Time) GatewayEffectiveState {
	if svc == nil || account == nil {
		return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
	}
	if account.IsShadow() {
		// Live selection additionally verifies the parent credential state.
		return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
	}

	if isOpenAIAccount(account) {
		if value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID); ok {
			until, valid := value.(time.Time)
			if !valid || until.IsZero() {
				return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
			}
			if now.Before(until) {
				return GatewayEffectiveState{gatewayEffectiveRuntimeTemporarilyUnavailable, "COOLDOWN_ACTIVE"}
			}
		}
	}

	if state := svc.openaiModelTransient; state != nil {
		canonicalModel := canonicalOpenAIAccountSchedulingModel(account, model)
		key, valid := openAIAccountModelTransientKey(account.ID, openAIAccountModelTransientModel(canonicalModel))
		if !valid {
			return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
		}
		state.mu.Lock()
		entry, ok := state.entries[key]
		state.mu.Unlock()
		if ok && !entry.lastFailure.IsZero() && now.Sub(entry.lastFailure) <= openAIModelTransientStreakTTL &&
			!entry.blockUntil.IsZero() && now.Before(entry.blockUntil) {
			return GatewayEffectiveState{gatewayEffectiveRuntimeTemporarilyUnavailable, "COOLDOWN_ACTIVE"}
		}
	}

	if account.ProxyID != nil && *account.ProxyID > 0 {
		// Inspecting the lazy circuit would initialize it; live selection also
		// has a fail-open retry, so a passive observer cannot make a final claim.
		return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
	}
	if !account.IsOpenAI() {
		return GatewayEffectiveState{State: gatewayEffectiveRuntimeAvailable}
	}

	if svc.settingService != nil {
		cached, _ := svc.settingService.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings)
		if cached != nil && now.UnixNano() < cached.expiresAt {
			ctx = withOpenAIQuotaAutoPauseSettings(ctx, cached.settings)
		}
	}
	if paused, _ := evaluateOpenAIAccountQuotaPause(ctx, account); paused {
		return GatewayEffectiveState{gatewayEffectiveRuntimeTemporarilyUnavailable, "QUOTA_TEMPORARILY_UNAVAILABLE"}
	}
	if svc.settingService != nil && !gatewayEffectiveQuotaSettingsKnown(ctx, account) {
		return GatewayEffectiveState{gatewayEffectiveRuntimeUnknown, "AVAILABILITY_UNKNOWN"}
	}
	return GatewayEffectiveState{State: gatewayEffectiveRuntimeAvailable}
}

func gatewayEffectiveQuotaSettingsKnown(ctx context.Context, account *Account) bool {
	if ctx != nil {
		if _, ok := ctx.Value(openAIQuotaAutoPauseCtxKey{}).(OpsOpenAIAccountQuotaAutoPauseSettings); ok {
			return true
		}
	}
	if account == nil {
		return false
	}
	for _, window := range []string{"5h", "7d"} {
		if resolveAccountExtraBool(account.Extra, "auto_pause_"+window+"_disabled") {
			continue
		}
		threshold, _ := resolveAccountExtraNumber(account.Extra, "auto_pause_"+window+"_threshold")
		if clamp01(threshold) <= 0 {
			return false
		}
	}
	return true
}
