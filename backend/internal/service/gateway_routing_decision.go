package service

import (
	"context"
	"sort"
	"time"
)

// GatewayRoutingDecision is a bounded explanation, never a selected route or
// admission promise. Multiplicity describes eligible backing candidates only.
type GatewayRoutingDecision struct {
	State               string   `json:"state"`
	Stage               string   `json:"stage"`
	Reason              string   `json:"reason,omitempty"`
	CandidateRoutes     string   `json:"candidate_routes"`
	AvailableRoutes     string   `json:"available_routes"`
	Retryability        string   `json:"retryability"`
	TransientConditions []string `json:"transient_conditions"`
}

func gatewayRouteMultiplicity(count int, known bool) string {
	if !known {
		return "unknown"
	}
	switch count {
	case 0:
		return "none"
	case 1:
		return "single"
	default:
		return "multiple"
	}
}

func gatewayRoutingDecision(request GatewayPreflightRequest, candidates gatewayEffectiveCandidates, shape bool) GatewayRoutingDecision {
	d := GatewayRoutingDecision{
		State: "indeterminate", Stage: "routing", Reason: "UNKNOWN",
		CandidateRoutes: "unknown", AvailableRoutes: "unknown", Retryability: "unknown",
		TransientConditions: []string{},
	}
	routing := candidates.view.Routing
	switch routing.State {
	case "restricted":
		d.State, d.Stage, d.Reason, d.Retryability = "blocked", "policy", routing.Reason, "change_configuration"
		d.CandidateRoutes, d.AvailableRoutes = "none", "none"
		return d
	case "not_configured":
		d.State, d.Reason, d.Retryability = "blocked", routing.Reason, "change_configuration"
		d.CandidateRoutes, d.AvailableRoutes = "none", "none"
		if candidates.reason == "PROTOCOL_UNSUPPORTED" {
			d.Stage, d.Reason, d.Retryability = "protocol", candidates.reason, "change_request"
		}
		return d
	case "configured":
	default:
		if candidates.reason == "REQUEST_DEPENDENT_POLICY_UNKNOWN" {
			d.Stage, d.Reason = "policy", candidates.reason
		}
		return d
	}

	eligible, available := 0, 0
	shapeKnown, availabilityKnown := true, true
	conditions := make(map[string]bool)
	for i, features := range candidates.configured {
		if shape {
			support := gatewayEvaluateRequest(request, features)
			if support.State == "unsupported" {
				continue
			}
			if support.State != "supported" {
				shapeKnown = false
				continue
			}
		}
		eligible++
		if i >= len(candidates.observations) || candidates.observations[i].State == "unknown" {
			availabilityKnown = false
			continue
		}
		observation := candidates.observations[i]
		if observation.State == "available" {
			available++
		} else if observation.Reason != "" {
			conditions[observation.Reason] = true
		}
	}
	d.CandidateRoutes = gatewayRouteMultiplicity(eligible, shapeKnown)
	d.AvailableRoutes = gatewayRouteMultiplicity(available, shapeKnown && availabilityKnown)
	for reason := range conditions {
		d.TransientConditions = append(d.TransientConditions, reason)
	}
	sort.Strings(d.TransientConditions)
	if shape && eligible == 0 && shapeKnown {
		d.State, d.Stage, d.Reason, d.Retryability = "blocked", "capability", gatewayEvaluateRequestSets(request, candidates.configured).Reason, "change_request"
		return d
	}
	if !shapeKnown {
		d.Stage, d.Reason = "capability", "UNKNOWN"
		return d
	}
	if !availabilityKnown {
		d.Stage, d.Reason = "availability", "AVAILABILITY_UNKNOWN"
		return d
	}
	if available == 0 {
		d.State, d.Stage, d.Reason = "blocked", "availability", "TEMPORARILY_UNAVAILABLE"
		// Generic missing-snapshot candidates do not prove that waiting helps.
		if len(conditions) > 0 && !conditions["TEMPORARILY_UNAVAILABLE"] && !conditions["QUOTA_UNAVAILABLE"] {
			d.Retryability = "retry_later"
		}
		if len(d.TransientConditions) == 1 {
			d.Reason = d.TransientConditions[0]
		}
		return d
	}
	d.State, d.Stage, d.Reason, d.Retryability = "constrained", "ready", "", "not_applicable"
	if len(candidates.configured) == 1 && eligible == 1 {
		d.State = "deterministic"
	}
	// Optional traits are not scheduler admission gates. Even a known usable
	// subset cannot promise which candidate the live scheduler will select.
	if shape && eligible != len(candidates.configured) {
		d.Reason = "ROUTE_DEPENDENT"
	}
	return d
}

// Explain only vetoes already established by the shared scheduling predicate.
// Never infer an upstream failure from a raw error string or an absent account.
func gatewayAccountSchedulingCondition(ctx context.Context, account *Account, model string, now time.Time) string {
	switch {
	case !account.IsActive() || !account.Schedulable || account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt):
		return "TEMPORARILY_UNAVAILABLE"
	case account.IsAPIKeyOrBedrock() && account.IsQuotaExceeded():
		// A total quota has no time-based recovery guarantee, even if a
		// simultaneous cooldown expires first.
		return "QUOTA_UNAVAILABLE"
	case account.OverloadUntil != nil && now.Before(*account.OverloadUntil):
		return "PROVIDER_TEMPORARILY_UNAVAILABLE"
	case account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt):
		return "RATE_LIMIT_ACTIVE"
	case account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil):
		return "COOLDOWN_ACTIVE"
	case account.IsSchedulable() && !account.IsSchedulableForModelWithContext(ctx, model):
		return "RATE_LIMIT_ACTIVE"
	default:
		return "TEMPORARILY_UNAVAILABLE"
	}
}

func gatewayCandidateAvailability(ctx context.Context, account *Account, current Account, found bool, model string, openAI *OpenAIGatewayService, now time.Time) GatewayEffectiveState {
	if !account.IsSchedulableForModelWithContext(ctx, model) {
		return GatewayEffectiveState{"temporarily_unavailable", gatewayAccountSchedulingCondition(ctx, account, model, now)}
	}
	if !found || current.Platform != account.Platform {
		return GatewayEffectiveState{"temporarily_unavailable", "TEMPORARILY_UNAVAILABLE"}
	}
	if !current.IsSchedulableForModelWithContext(ctx, model) {
		return GatewayEffectiveState{"temporarily_unavailable", gatewayAccountSchedulingCondition(ctx, &current, model, now)}
	}
	if account.IsOpenAICompatible() {
		return gatewayEffectiveOpenAIRuntimeObservation(ctx, openAI, account, model, now)
	}
	return GatewayEffectiveState{State: "available"}
}
