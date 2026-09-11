package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayEffectivePreflightCasesArePassive(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformAntigravity, "public-model", "upstream-model")
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	snapshot := gatewayCapabilitySchedulerSnapshot(42, PlatformAntigravity, &account)
	var upstreamCalls atomic.Int64
	upstream := &codexModelsHTTPUpstreamStub{do: func(*http.Request, string, int64, int) (*http.Response, error) {
		upstreamCalls.Add(1)
		return nil, fmt.Errorf("unexpected inference")
	}}
	gateway := &GatewayService{accountRepo: repo, schedulerSnapshot: snapshot, httpUpstream: upstream}
	group := &Group{ID: 42, Platform: PlatformAntigravity}
	before, err := json.Marshal(repo.configured)
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		tools []string
		state string
	}{
		{"web", []string{"web_search"}, "supported"},
		{"function", []string{"functions"}, "supported"},
		{"mixed", []string{"web_search", "functions"}, "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: "public-model", Protocol: "responses", Tools: tc.tools}, nil)
			require.NoError(t, err)
			require.Equal(t, tc.state, result.Support.State)
			if tc.state == "unsupported" {
				require.Equal(t, "CAPABILITY_COMBINATION_UNSUPPORTED", result.Support.Reason)
			}
			payload, err := json.Marshal(result)
			require.NoError(t, err)
			t.Logf("GATEWAY_SCHEMA_RESPONSE %s", payload)
		})
	}
	payload, err := json.Marshal(gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil))
	require.NoError(t, err)
	t.Logf("GATEWAY_SCHEMA_CAPABILITIES %s", payload)
	after, err := json.Marshal(repo.configured)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	require.Zero(t, upstreamCalls.Load())
	require.Zero(t, repo.currentCalls)
	cache, ok := snapshot.cache.(*gatewayCapabilitySchedulerCacheStub)
	require.True(t, ok)
	require.Zero(t, cache.setCalls)
	require.Zero(t, cache.captureCalls)
}

func TestGatewayEffectiveFutureManifestAndPolicy(t *testing.T) {
	account := gatewayCapabilityTestAccount(1, PlatformOpenAI, nil)
	account.Type = AccountTypeOAuth
	account.Extra[codexFingerprintSeedExtraKey] = "11111111-1111-4111-8111-111111111111"
	setCodexManifestSnapshotForTest(&account, "test", `{"models":[
		{"slug":"gpt-future-codex-model","supported_in_api":true},
		{"slug":"gpt-future-hidden","visibility":"hidden"},
		{"slug":"gpt-future-disabled","supported_in_api":false}
	]}`, time.Now())
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	gateway := &GatewayService{accountRepo: repo}
	group := &Group{ID: 42, Platform: PlatformOpenAI}
	result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	require.Len(t, result.Models, 1)
	require.Equal(t, "gpt-future-codex-model", result.Models[0].ID)
	require.Equal(t, "observed", result.Models[0].Catalog.Discovery)
	require.Equal(t, "configured", result.Models[0].Protocols["responses"].Routing.State)
	require.Equal(t, "unknown", result.Models[0].Protocols["responses"].Capabilities.Features["reasoning"])
	require.Equal(t, "unknown", result.Models[0].Protocols["responses"].Capabilities.Features["text_input"])
	group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"another"}}
	require.Empty(t, gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil).Models)
	group.ModelAllowlist.Enabled = false
	account.Credentials["model_mapping"] = map[string]any{"operator-alias": "gpt-future-codex-model"}
	repo.configured = []Account{account}
	result = gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	require.Len(t, result.Models, 1)
	require.Equal(t, "operator-alias", result.Models[0].ID)
}

func TestGatewayEffectiveTransientCasesPreserveCatalog(t *testing.T) {
	for _, kind := range []string{"healthy", "rate_limit", "overload", "cooldown", "missing_snapshot"} {
		t.Run(kind, func(t *testing.T) {
			account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "public", "public")
			until := time.Now().Add(time.Hour)
			switch kind {
			case "rate_limit":
				account.RateLimitResetAt = &until
			case "overload":
				account.OverloadUntil = &until
			case "cooldown":
				account.TempUnschedulableUntil = &until
			}
			gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}, schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformAnthropic, &account)}
			if kind == "missing_snapshot" {
				gateway.schedulerSnapshot = nil
			}
			result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), &Group{ID: 42, Platform: PlatformAnthropic}, nil)
			require.Len(t, result.Models, 1)
			require.True(t, result.Models[0].Catalog.Published)
			require.Equal(t, "configured", result.Models[0].Protocols["responses"].Routing.State)
			want := "temporarily_unavailable"
			if kind == "healthy" {
				want = "available"
			}
			if kind == "missing_snapshot" {
				want = "unknown"
			}
			require.Equal(t, want, result.Models[0].Protocols["responses"].Availability.State)
		})
	}
}

func TestGatewayEffectiveRuntimeConcurrentObservation(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	svc := &OpenAIGatewayService{openaiModelTransient: newOpenAIAccountModelTransientState(8)}
	model := canonicalOpenAIAccountSchedulingModel(account, "future-model")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			svc.openaiAccountRuntimeBlockUntil.Store(account.ID, time.Now().Add(time.Minute))
			svc.openaiModelTransient.recordFailure(account.ID, model, time.Now())
			svc.openaiModelTransient.recordSuccess(account.ID, model)
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			gatewayEffectiveOpenAIRuntimeState(context.Background(), svc, account, "future-model")
		}
	}()
	wg.Wait()
}

func BenchmarkGatewayEffectiveCapabilities(b *testing.B) {
	mapping := map[string]string{}
	metadata := map[string]UpstreamModelMetadata{}
	for i := range 50 {
		model := fmt.Sprintf("future-%d", i)
		mapping[model] = model
		metadata[model] = UpstreamModelMetadata{ID: model, InputModalities: []string{"text"}, ContextWindow: 128000}
	}
	accounts := make([]Account, 10)
	for i := range accounts {
		accounts[i] = gatewayCapabilityTestAccount(int64(i+1), PlatformOpenAI, mapping)
		accounts[i].SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: metadata})
	}
	repo := &gatewayCapabilityAccountRepoStub{configured: accounts}
	gateway := &GatewayService{accountRepo: repo}
	group := &Group{ID: 42, Platform: PlatformOpenAI}
	b.ReportAllocs()
	for b.Loop() {
		gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	}
	if repo.configuredCalls != b.N {
		b.Fatalf("expected one pool read per request, got %d", repo.configuredCalls)
	}
}

func TestGatewayEffectiveCompositeUsesSchedulerMixedEligibility(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformAntigravity, "upstream-model", "upstream-model")
	account.Extra["mixed_scheduling"] = true
	routes := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{{ID: 1, PublicModel: "public-alias", UpstreamModel: "upstream-model", TargetPlatform: PlatformAnthropic, Endpoint: "responses", MatchType: "exact", Enabled: true}}}
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	gateway := &GatewayService{accountRepo: repo, compositeResolver: NewCompositeRouteResolver(routes), schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformAnthropic, &account)}
	group := &Group{ID: 42, Platform: PlatformComposite}
	require.True(t, gateway.isAccountAllowedForPlatform(&account, PlatformAnthropic, true))
	result, err := gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: "public-alias", Protocol: "responses", Tools: []string{"functions"}}, nil)
	require.NoError(t, err)
	require.True(t, result.Catalog.Published)
	require.Equal(t, "configured", result.Routing.State)
	require.Equal(t, "supported", result.Support.State)
	require.Equal(t, "available", result.Availability.State)
	account.Extra["mixed_scheduling"] = false
	repo.configured = []Account{account}
	result, err = gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: "public-alias", Protocol: "responses"}, nil)
	require.NoError(t, err)
	require.Equal(t, "not_configured", result.Routing.State)
}

func TestGatewayEffectiveReasoningEffortRequiresReasoningEvidence(t *testing.T) {
	features := gatewayUnknownFeatures()
	features.Features["protocol"] = "supported"
	features.ReasoningEfforts = []string{"low"}
	request := GatewayPreflightRequest{ReasoningEffort: "low"}
	require.Equal(t, "unknown", gatewayEvaluateRequest(request, features).State)
	features.Features["reasoning"] = "unsupported"
	require.Equal(t, "unsupported", gatewayEvaluateRequest(request, features).State)
	features.Features["reasoning"] = "supported"
	require.Equal(t, "supported", gatewayEvaluateRequest(request, features).State)
}

func TestGatewayEffectiveGeminiChatRejectsMixedAccount(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformAntigravity, "upstream-model", "upstream-model")
	account.Extra["mixed_scheduling"] = true
	routes := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{{ID: 1, PublicModel: "public-alias", UpstreamModel: "upstream-model", TargetPlatform: PlatformGemini, Endpoint: "chat_completions", MatchType: "exact", Enabled: true}}}
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}, compositeResolver: NewCompositeRouteResolver(routes)}
	require.False(t, GatewayChatAccountCompatible(PlatformGemini, &account))
	result, err := gateway.PreflightGatewayRequest(context.Background(), &Group{ID: 42, Platform: PlatformComposite}, GatewayPreflightRequest{SchemaVersion: 2, Model: "public-alias", Protocol: "chat_completions"}, nil)
	require.NoError(t, err)
	require.Equal(t, "not_configured", result.Routing.State)
}

func TestGatewayEffectiveDynamicDiscoveryUsesRequestSnapshot(t *testing.T) {
	account := gatewayCapabilityTestAccount(1, PlatformOpenAI, nil)
	account.Type = AccountTypeOAuth
	account.Extra[codexFingerprintSeedExtraKey] = "11111111-1111-4111-8111-111111111111"
	setCodexManifestSnapshotForTest(&account, "test", `{"models":[{"slug":"future-one"},{"slug":"future-two"}]}`, time.Now())
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
	group := &Group{ID: 42, Platform: PlatformOpenAI}
	snapshot := gateway.loadGatewayEffectiveSnapshot(context.Background(), group)
	// Observation is decoded once. Removing the original payload afterwards
	// must not change the already-captured request's discovery evidence.
	delete(account.Extra, OpenAICodexManifestSnapshotExtraKey)
	for _, model := range []string{"future-one", "future-two"} {
		catalog := gateway.gatewayEffectiveCatalog(context.Background(), group, model, true, snapshot)
		require.Equal(t, "observed", catalog.Discovery)
	}
}

func TestGatewayEffectiveConstraintsCoverRequestTraits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		feature string
		request GatewayPreflightRequest
	}{
		{"input", "image_input", GatewayPreflightRequest{InputModalities: []string{"image"}}},
		{"stream", "streaming", GatewayPreflightRequest{Streaming: true}},
		{"tier", "service_tier.priority", GatewayPreflightRequest{ServiceTier: "priority"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			features := gatewayUnknownFeatures()
			features.Features["protocol"] = "supported"
			features.Features["functions"] = "supported"
			features.Features[tc.feature] = "supported"
			features.Constraints = []GatewayCapabilityConstraint{{AllOf: []string{"functions", tc.feature}, Reason: "CAPABILITY_COMBINATION_UNSUPPORTED"}}
			require.Equal(t, "supported", gatewayEvaluateRequest(tc.request, features).State)
			tc.request.Tools = []string{"functions"}
			require.Equal(t, GatewayEffectiveState{"unsupported", "CAPABILITY_COMBINATION_UNSUPPORTED"}, gatewayEvaluateRequest(tc.request, features))
		})
	}
}
