package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func gatewayEffectiveTestAccount(id int64, platform, public, upstream string) Account {
	account := gatewayCapabilityTestAccount(id, platform, map[string]string{public: upstream})
	account.SetUpstreamModelInventorySnapshot(UpstreamModelInventorySnapshot{Source: "upstream", Models: []string{upstream}})
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Source: "upstream", Models: map[string]UpstreamModelMetadata{
		upstream: {ID: upstream, InputModalities: []string{"text"}},
	}})
	return account
}

func TestGatewayEffectiveSeparatesCatalogRoutingAndTransientState(t *testing.T) {
	ctx := context.Background()
	account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "future-model", "future-model")
	until := time.Now().Add(time.Hour)
	account.RateLimitResetAt = &until
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	snapshot := gatewayCapabilitySchedulerSnapshot(42, PlatformAnthropic, &account)
	cache, ok := snapshot.cache.(*gatewayCapabilitySchedulerCacheStub)
	require.True(t, ok)
	gateway := &GatewayService{accountRepo: repo, schedulerSnapshot: snapshot}
	group := &Group{ID: 42, Platform: PlatformAnthropic}

	result := gateway.BuildGatewayEffectiveCapabilities(ctx, group, nil)
	require.Len(t, result.Models, 1)
	model := result.Models[0]
	require.True(t, model.Catalog.Published)
	require.Equal(t, "observed", model.Catalog.Discovery)
	require.Equal(t, "configured", model.Protocols["responses"].Routing.State)
	require.Equal(t, "temporarily_unavailable", model.Protocols["responses"].Availability.State)
	require.Equal(t, "supported", model.Protocols["responses"].Capabilities.Features["text_input"])
	require.Equal(t, 1, repo.configuredCalls)
	require.Zero(t, repo.currentCalls)
	require.Zero(t, cache.captureCalls)
	require.Zero(t, cache.setCalls)

	gateway.schedulerSnapshot = nil
	result = gateway.BuildGatewayEffectiveCapabilities(ctx, group, nil)
	require.Equal(t, "configured", result.Models[0].Protocols["responses"].Routing.State)
	require.Equal(t, "unknown", result.Models[0].Protocols["responses"].Availability.State)
}

func TestGatewayEffectiveDisplayDoesNotEnforceAndAllowlistDoes(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "hidden-model", "hidden-model")
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	gateway := &GatewayService{accountRepo: repo, schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformAnthropic, &account)}
	group := &Group{ID: 42, Platform: PlatformAnthropic, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"other"}}}
	request := GatewayPreflightRequest{SchemaVersion: 2, Model: "hidden-model", Protocol: "responses", InputModalities: []string{"text"}}
	result, err := gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
	require.NoError(t, err)
	require.False(t, result.Catalog.Published)
	require.Equal(t, "configured", result.Routing.State)
	require.Equal(t, "supported", result.Support.State)
	require.Equal(t, "available", result.Availability.State)

	group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}
	calls := repo.configuredCalls
	result, err = gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
	require.NoError(t, err)
	require.Equal(t, "restricted", result.Routing.State)
	require.Equal(t, "MODEL_NOT_ALLOWED", result.Support.Reason)
	require.Equal(t, calls, repo.configuredCalls, "denial must not read unrelated account evidence")
	require.Empty(t, gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil).Models)
}

func TestGatewayEffectiveCompositeEndpointOwnership(t *testing.T) {
	vision := gatewayEffectiveTestAccount(1, PlatformOpenAI, "vision-upstream", "vision-upstream")
	vision.Credentials["openai_capabilities"] = []string{"chat_completions", "vision_input"}
	vision.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{
		"vision-upstream": {ID: "vision-upstream", InputModalities: []string{"text", "image"}},
	}})
	tools := gatewayEffectiveTestAccount(2, PlatformAntigravity, "tools-upstream", "tools-upstream")
	routes := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 1, PublicModel: "shared", TargetPlatform: PlatformOpenAI, UpstreamModel: "vision-upstream", Endpoint: "responses", MatchType: "exact", Enabled: true},
		{ID: 2, PublicModel: "shared", TargetPlatform: PlatformAntigravity, UpstreamModel: "tools-upstream", Endpoint: "messages", MatchType: "exact", Enabled: true},
		{ID: 3, PublicModel: "messages-only", TargetPlatform: PlatformAntigravity, UpstreamModel: "tools-upstream", Endpoint: "messages", MatchType: "exact", Enabled: true},
		{ID: 4, PublicModel: "disabled", TargetPlatform: PlatformAntigravity, UpstreamModel: "tools-upstream", Endpoint: "messages", MatchType: "exact", Enabled: false},
	}}
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{vision, tools}}, compositeResolver: NewCompositeRouteResolver(routes)}
	group := &Group{ID: 42, Platform: PlatformComposite}
	result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	byID := make(map[string]GatewayEffectiveModel)
	for _, model := range result.Models {
		byID[model.ID] = model
	}
	require.Contains(t, byID, "messages-only")
	require.NotContains(t, byID, "disabled")
	require.Equal(t, "supported", byID["shared"].Protocols["responses"].Capabilities.Features["image_input"])
	require.NotEqual(t, "supported", byID["shared"].Protocols["responses"].Capabilities.Features["functions"])
	require.Equal(t, "supported", byID["shared"].Protocols["messages"].Capabilities.Features["functions"])
	require.NotEqual(t, "supported", byID["shared"].Protocols["messages"].Capabilities.Features["image_input"])

	preflight, err := gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{
		SchemaVersion: 2, Model: "shared", Protocol: "messages", Tools: []string{"functions", "web_search"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "unsupported", preflight.Support.State)
	require.Equal(t, "CAPABILITY_COMBINATION_UNSUPPORTED", preflight.Support.Reason)
	require.Equal(t, "not_applicable", preflight.Availability.State)
}

func TestGatewayEffectiveNoStaleSchedulerPublicationOrCapabilityUnion(t *testing.T) {
	old := gatewayEffectiveTestAccount(1, PlatformAnthropic, "deleted", "deleted")
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{}, schedulerSnapshot: gatewayCapabilitySchedulerSnapshot(42, PlatformAnthropic, &old)}
	require.Empty(t, gateway.BuildGatewayEffectiveCapabilities(context.Background(), &Group{ID: 42, Platform: PlatformAnthropic}, nil).Models)

	vision := gatewayUnknownFeatures()
	vision.Features["protocol"] = "supported"
	vision.Features["image_input"] = "supported"
	vision.Features["functions"] = "unsupported"
	functions := gatewayUnknownFeatures()
	functions.Features["protocol"] = "supported"
	functions.Features["image_input"] = "unsupported"
	functions.Features["functions"] = "supported"
	request := GatewayPreflightRequest{Protocol: "responses", InputModalities: []string{"image"}, Tools: []string{"functions"}}
	require.Equal(t, "unsupported", gatewayEvaluateRequestSets(request, []GatewayFeatureSet{vision, functions}).State)
	request.InputModalities = nil
	require.Equal(t, "conditional", gatewayEvaluateRequestSets(request, []GatewayFeatureSet{vision, functions}).State)
}

func TestGatewayEffectiveUnknownReadsAndSimpleScope(t *testing.T) {
	account := gatewayEffectiveTestAccount(1, PlatformOpenAI, "future-model", "future-model")
	repo := &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	gateway := &GatewayService{accountRepo: repo, cfg: &config.Config{RunMode: config.RunModeSimple}}
	group := &Group{ID: 42, Platform: PlatformOpenAI}
	result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	require.True(t, repo.configuredGroupIDNil)
	require.True(t, repo.configuredIncludeGrouped)
	require.Equal(t, "configured", result.Models[0].Protocols["responses"].Routing.State)

	repo.configuredErr = errors.New("private database failure")
	result = gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	require.Equal(t, "unknown", result.CatalogState)
	require.Empty(t, result.Models)
}

func TestGatewayEffectiveCompositeAmbiguityAndObservedAlias(t *testing.T) {
	first := gatewayEffectiveTestAccount(1, PlatformAnthropic, "shared", "shared")
	second := gatewayEffectiveTestAccount(2, PlatformOpenAI, "shared", "shared")
	routes := &gatewayCapabilityRouteRepoStub{}
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{first, second}}, compositeResolver: NewCompositeRouteResolver(routes)}
	group := &Group{ID: 42, Platform: PlatformComposite}
	require.Empty(t, gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil).Models)

	routes.routes = []CompositeModelRoute{{ID: 1, Enabled: true, MatchType: "exact", Endpoint: "responses", PublicModel: "alias", TargetPlatform: PlatformAnthropic, UpstreamModel: "shared"}}
	result := gateway.BuildGatewayEffectiveCapabilities(context.Background(), group, nil)
	require.Len(t, result.Models, 1)
	require.Equal(t, "alias", result.Models[0].ID)
	require.Equal(t, "observed", result.Models[0].Catalog.Discovery)
}

func TestGatewayEffectiveProtocolOperatorGates(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformKimi} {
		t.Run(platform, func(t *testing.T) {
			account := gatewayEffectiveTestAccount(1, platform, "public", "public")
			gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
			group := &Group{ID: 42, Platform: platform}
			request := GatewayPreflightRequest{SchemaVersion: 2, Model: "public", Protocol: "messages"}
			result, err := gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
			require.NoError(t, err)
			if platform == PlatformOpenAI {
				require.Equal(t, "restricted", result.Routing.State)
				group.AllowMessagesDispatch = true
				result, err = gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
				require.NoError(t, err)
			}
			require.Equal(t, "configured", result.Routing.State)
		})
	}
	account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "public", "public")
	gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}}
	group := &Group{ID: 42, Platform: PlatformAnthropic, ClaudeCodeOnly: true}
	result, err := gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: "public", Protocol: "responses"}, nil)
	require.NoError(t, err)
	require.Equal(t, "restricted", result.Routing.State)
	group.ClaudeCodeOnly = false
	group.RequirePrivacySet = true
	group.Platform = PlatformOpenAI
	account = gatewayEffectiveTestAccount(1, PlatformOpenAI, "public", "public")
	gateway.accountRepo = &gatewayCapabilityAccountRepoStub{configured: []Account{account}}
	result, err = gateway.PreflightGatewayRequest(context.Background(), group, GatewayPreflightRequest{SchemaVersion: 2, Model: "public", Protocol: "responses"}, nil)
	require.NoError(t, err)
	require.Equal(t, "restricted", result.Routing.State)
}

func TestGatewayEffectivePreflightReasonsMatchPublishedSchemas(t *testing.T) {
	unsupported := gatewayUnknownFeatures()
	unsupported.Features["protocol"] = "supported"
	unsupported.Features["image_input"] = "unsupported"
	supported := gatewayUnknownFeatures()
	supported.Features["protocol"] = "supported"
	supported.Features["image_input"] = "supported"
	request := GatewayPreflightRequest{InputModalities: []string{"image"}}
	results := []GatewayEffectiveState{
		gatewayEvaluateRequestSets(request, []GatewayFeatureSet{unsupported}),
		gatewayEvaluateRequestSets(request, []GatewayFeatureSet{unsupported, supported}),
	}
	for _, filename := range []string{"integration-contract-v2.schema.json", "gateway-preflight-v2.schema.json"} {
		body, err := os.ReadFile("../../../docs/" + filename)
		require.NoError(t, err)
		var schema struct {
			Defs map[string]struct {
				Enum []string `json:"enum"`
			} `json:"$defs"`
		}
		require.NoError(t, json.Unmarshal(body, &schema))
		for _, result := range results {
			require.Contains(t, schema.Defs["reason"].Enum, result.Reason, filename)
		}
	}
}
