package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayRoutingDecisionPreflight(t *testing.T) {
	for _, tc := range []struct {
		name, state, stage, reason, candidates, available, retry string
	}{
		{"healthy", "deterministic", "ready", "", "single", "single", "not_applicable"},
		{"multiple", "constrained", "ready", "", "multiple", "multiple", "not_applicable"},
		{"rate_limit", "blocked", "availability", "RATE_LIMIT_ACTIVE", "single", "none", "retry_later"},
		{"overload", "blocked", "availability", "PROVIDER_TEMPORARILY_UNAVAILABLE", "single", "none", "retry_later"},
		{"cooldown", "blocked", "availability", "COOLDOWN_ACTIVE", "single", "none", "retry_later"},
		{"snapshot_miss", "indeterminate", "availability", "AVAILABILITY_UNKNOWN", "single", "unknown", "unknown"},
		{"empty_snapshot", "blocked", "availability", "TEMPORARILY_UNAVAILABLE", "single", "none", "unknown"},
		{"policy", "blocked", "policy", "MODEL_NOT_ALLOWED", "none", "none", "change_configuration"},
		{"request_policy", "indeterminate", "policy", "REQUEST_DEPENDENT_POLICY_UNKNOWN", "unknown", "unknown", "unknown"},
		{"not_configured", "blocked", "routing", "NO_CONFIGURED_ROUTE", "none", "none", "change_configuration"},
		{"total_quota", "blocked", "availability", "QUOTA_UNAVAILABLE", "single", "none", "unknown"},
		{"protocol", "blocked", "protocol", "PROTOCOL_UNSUPPORTED", "none", "none", "change_request"},
		{"capability", "blocked", "capability", "CAPABILITY_UNSUPPORTED", "none", "none", "change_request"},
		{"combination", "blocked", "capability", "CAPABILITY_COMBINATION_UNSUPPORTED", "none", "none", "change_request"},
		{"unknown_capability", "indeterminate", "capability", "UNKNOWN", "unknown", "unknown", "unknown"},
		{"messages_policy", "blocked", "policy", "OPERATOR_RESTRICTED", "none", "none", "change_configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := gatewayEffectiveTestAccount(901, PlatformAnthropic, "public", "private-upstream")
			group := &Group{ID: 42, Platform: PlatformAnthropic}
			request := GatewayPreflightRequest{SchemaVersion: 2, Model: "public", Protocol: "responses", InputModalities: []string{"text"}}
			until := time.Now().Add(24 * time.Hour)
			switch tc.name {
			case "rate_limit":
				account.RateLimitResetAt = &until
			case "overload":
				account.OverloadUntil = &until
			case "cooldown":
				account.TempUnschedulableUntil = &until
			case "total_quota":
				account.Type = AccountTypeAPIKey
				account.Extra["quota_limit"], account.Extra["quota_used"] = 10.0, 10.0
			case "policy":
				group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"other"}}
			case "request_policy":
				group.ProfitControlEnabled = true
			case "not_configured":
				request.Model = "unmapped"
			case "protocol", "capability", "messages_policy":
				account.Platform, group.Platform = PlatformOpenAI, PlatformOpenAI
				account.Credentials["openai_capabilities"] = []string{"chat_completions"}
				if tc.name == "protocol" {
					account.Credentials["openai_capabilities"] = []string{"responses"}
				}
				if tc.name == "capability" {
					request.InputModalities = []string{"image"}
				}
				if tc.name == "messages_policy" {
					request.Protocol = "messages"
				}
			case "combination":
				account.Platform, group.Platform = PlatformAntigravity, PlatformAntigravity
				request.Tools = []string{"web_search", "functions"}
			case "unknown_capability":
				request.Tools = []string{"functions"}
			}
			accounts := []Account{account}
			current := []*Account{&account}
			if tc.name == "multiple" {
				other := gatewayEffectiveTestAccount(902, account.Platform, "public", "another-private-upstream")
				accounts = append(accounts, other)
				current = append(current, &other)
			}
			if tc.name == "empty_snapshot" {
				current = nil
			}
			repo := &gatewayCapabilityAccountRepoStub{configured: accounts}
			gateway := &GatewayService{accountRepo: repo, schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, group.Platform, current...)}
			if tc.name == "snapshot_miss" {
				gateway.schedulerSnapshot = nil
			}
			result, err := gateway.PreflightGatewayRequest(context.Background(), group, request, &OpenAIGatewayService{})
			require.NoError(t, err)
			d := result.Decision
			require.Equal(t, tc.state, d.State)
			require.Equal(t, tc.stage, d.Stage)
			require.Equal(t, tc.reason, d.Reason)
			require.Equal(t, tc.candidates, d.CandidateRoutes)
			require.Equal(t, tc.available, d.AvailableRoutes)
			require.Equal(t, tc.retry, d.Retryability)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			for _, secret := range []string{"901", "902", "private-upstream", "account_id", "group_id", "credentials", "reset_at"} {
				// generated_at contains arbitrary digits; inspect only the explanation for numeric IDs.
				decisionJSON, marshalErr := json.Marshal(d)
				require.NoError(t, marshalErr)
				require.NotContains(t, string(decisionJSON), secret)
			}
			require.NotContains(t, string(encoded), "private-upstream")
			if tc.name == "policy" {
				require.Zero(t, repo.configuredCalls)
			} else {
				require.Equal(t, 1, repo.configuredCalls)
			}
			require.Zero(t, repo.currentCalls)
		})
	}
}

func TestGatewayRoutingDecisionWholeShapeAndStableSerialization(t *testing.T) {
	vision, functions := gatewayUnknownFeatures(), gatewayUnknownFeatures()
	for _, set := range []GatewayFeatureSet{vision, functions} {
		set.Features["protocol"] = "supported"
	}
	vision.Features["image_input"], vision.Features["functions"] = "supported", "unsupported"
	functions.Features["image_input"], functions.Features["functions"] = "unsupported", "supported"
	candidates := gatewayEffectiveCandidates{
		configured:   []GatewayFeatureSet{vision, functions},
		observations: []GatewayEffectiveState{{"temporarily_unavailable", "COOLDOWN_ACTIVE"}, {State: "available"}},
		view:         GatewayEffectiveProtocol{Routing: GatewayEffectiveState{State: "configured"}},
	}
	request := GatewayPreflightRequest{Tools: []string{"functions"}, InputModalities: []string{"image"}}
	d := gatewayRoutingDecision(request, candidates, true)
	require.Equal(t, "none", d.CandidateRoutes)
	require.Equal(t, "blocked", d.State)
	require.Empty(t, d.TransientConditions, "do not report runtime conditions of incompatible candidates")
	request.InputModalities = nil
	d = gatewayRoutingDecision(request, candidates, true)
	require.Equal(t, "single", d.AvailableRoutes)
	require.Equal(t, "constrained", d.State)
	require.Equal(t, "ROUTE_DEPENDENT", d.Reason)
	require.Empty(t, d.TransientConditions)
	request.Tools = nil
	candidates.observations[1] = GatewayEffectiveState{"temporarily_unavailable", "RATE_LIMIT_ACTIVE"}
	first, err := json.Marshal(gatewayRoutingDecision(request, candidates, true))
	require.NoError(t, err)
	candidates.configured[0], candidates.configured[1] = candidates.configured[1], candidates.configured[0]
	candidates.observations[0], candidates.observations[1] = candidates.observations[1], candidates.observations[0]
	second, err := json.Marshal(gatewayRoutingDecision(request, candidates, true))
	require.NoError(t, err)
	require.Equal(t, first, second)
	candidates.observations[0] = GatewayEffectiveState{"unknown", "AVAILABILITY_UNKNOWN"}
	d = gatewayRoutingDecision(request, candidates, true)
	require.Equal(t, "unknown", d.AvailableRoutes)
	require.Equal(t, "unknown", d.Retryability)
}

func TestGatewayRoutingDecisionRuntimeObservation(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc := &OpenAIGatewayService{openaiModelTransient: newOpenAIAccountModelTransientState(8)}
	svc.openaiAccountRuntimeBlockUntil.Store(account.ID, now.Add(time.Minute))
	require.Equal(t, "COOLDOWN_ACTIVE", gatewayEffectiveOpenAIRuntimeObservation(context.Background(), svc, account, "future", now).Reason)
	require.Equal(t, "available", gatewayEffectiveOpenAIRuntimeObservation(context.Background(), svc, account, "future", now.Add(time.Hour)).State)
	_, exists := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, exists, "observation must not prune expired runtime state")
}

func TestGatewayRoutingDecisionSchemaEnums(t *testing.T) {
	for _, filename := range []string{"integration-contract-v2.schema.json", "gateway-preflight-v2.schema.json"} {
		body, err := os.ReadFile("../../../docs/" + filename)
		require.NoError(t, err)
		var schema struct {
			Defs map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"$defs"`
		}
		require.NoError(t, json.Unmarshal(body, &schema))
		for _, reason := range []string{"UNKNOWN", "MODEL_NOT_ALLOWED", "OPERATOR_RESTRICTED", "NO_CONFIGURED_ROUTE", "PROTOCOL_UNSUPPORTED", "REQUEST_DEPENDENT_POLICY_UNKNOWN", "CAPABILITY_UNSUPPORTED", "CAPABILITY_COMBINATION_UNSUPPORTED", "ROUTE_DEPENDENT", "AVAILABILITY_UNKNOWN", "TEMPORARILY_UNAVAILABLE", "RATE_LIMIT_ACTIVE", "COOLDOWN_ACTIVE", "PROVIDER_TEMPORARILY_UNAVAILABLE", "QUOTA_TEMPORARILY_UNAVAILABLE", "QUOTA_UNAVAILABLE"} {
			require.Contains(t, schema.Defs["decision"].Properties["reason"].Enum, reason)
		}
	}
}

func TestGatewayRoutingDecisionPublishedWithoutRoute(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "mapped", "upstream")
	model := DefaultGatewayCapabilityFallbacks()[PlatformAnthropic][0]
	group := &Group{ID: 42, Platform: PlatformAnthropic, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{model}}}
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
	result, err := gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: model, Protocol: "responses"}, nil)
	require.NoError(t, err)
	require.True(t, result.Catalog.Published)
	require.Equal(t, "none", result.Decision.CandidateRoutes)
	require.Equal(t, "NO_CONFIGURED_ROUTE", result.Decision.Reason)
}

func TestGatewayRoutingDecisionQuotaObservationDoesNotNotify(t *testing.T) {
	now := time.Now()
	account := gatewayEffectiveTestAccount(1, PlatformOpenAI, "public", "upstream")
	account.Type = AccountTypeOAuth
	account.Extra["codex_5h_used_percent"] = 100.0
	account.Extra["codex_usage_updated_at"] = now.Format(time.RFC3339)
	account.Extra["codex_5h_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
	account.Extra[OpenAIAutoResetCreditEnabledExtraKey] = true
	account.Extra[OpenAIAutoResetCredit5hThresholdExtraKey] = 1.0
	notifier := &OpenAIQuotaAutoResetService{ctx: context.Background(), queue: make(chan int64, 1)}
	setOpenAIAutoResetNotifier(notifier)
	t.Cleanup(func() { clearOpenAIAutoResetNotifier(notifier) })
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}, schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformOpenAI, &account)}
	result, err := gateway.PreflightGatewayRequest(context.Background(), &Group{ID: 42, Platform: PlatformOpenAI}, GatewayPreflightRequest{SchemaVersion: 2, Model: "public", Protocol: "responses"}, &OpenAIGatewayService{})
	require.NoError(t, err)
	require.Equal(t, "QUOTA_TEMPORARILY_UNAVAILABLE", result.Decision.Reason)
	require.Equal(t, "retry_later", result.Decision.Retryability)
	require.Empty(t, notifier.queue)
}

func TestGatewayRoutingDecisionDurableVetoPreventsRetryAdvice(t *testing.T) {
	now := time.Now()
	expired, until := now.Add(-time.Hour), now.Add(time.Hour)
	for _, kind := range []string{"expired", "disabled", "total_quota"} {
		t.Run(kind, func(t *testing.T) {
			account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "public", "upstream")
			account.RateLimitResetAt = &until
			reason := "TEMPORARILY_UNAVAILABLE"
			switch kind {
			case "expired":
				account.AutoPauseOnExpired, account.ExpiresAt = true, &expired
			case "disabled":
				account.Schedulable = false
			case "total_quota":
				account.Type = AccountTypeAPIKey
				account.Extra["quota_limit"], account.Extra["quota_used"] = 10.0, 10.0
				reason = "QUOTA_UNAVAILABLE"
			}
			observation := gatewayCandidateAvailability(context.Background(), &account, account, true, "public", nil, now)
			require.Equal(t, reason, observation.Reason)
			candidates := gatewayEffectiveCandidates{configured: []GatewayFeatureSet{gatewayAccountFeatures(&account, "upstream", "responses")}, observations: []GatewayEffectiveState{observation}, view: GatewayEffectiveProtocol{Routing: GatewayEffectiveState{State: "configured"}}}
			require.Equal(t, "unknown", gatewayRoutingDecision(GatewayPreflightRequest{Protocol: "responses"}, candidates, true).Retryability)
		})
	}
}
