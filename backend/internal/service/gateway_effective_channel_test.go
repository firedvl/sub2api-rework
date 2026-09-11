package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayEffectiveChannelMappingAndRestrictions(t *testing.T) {
	for _, source := range []string{BillingModelSourceRequested, BillingModelSourceChannelMapped, BillingModelSourceUpstream} {
		t.Run(source, func(t *testing.T) {
			account := gatewayEffectiveTestAccount(1, PlatformAnthropic, "channel-target", "account-target")
			channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{42}, RestrictModels: true, BillingModelSource: source,
				ModelMapping: map[string]map[string]string{PlatformAnthropic: {"composite-target": "channel-target"}},
				ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"unrelated"}}},
			}
			cache := populateChannelCache([]Channel{channel}, map[int64]string{42: PlatformComposite})
			cache.loadedAt = time.Now()
			channelService := &ChannelService{}
			channelService.cache.Store(cache)
			routeRepo := &gatewayCapabilityRouteRepoStub{routes: []CompositeModelRoute{{ID: 1, Enabled: true, MatchType: "exact", Endpoint: "responses", PublicModel: "public-alias", TargetPlatform: PlatformAnthropic, UpstreamModel: "composite-target"}}}
			gateway := &GatewayService{accountRepo: &gatewayCapabilityAccountRepoStub{configured: []Account{account}}, channelService: channelService, compositeResolver: NewCompositeRouteResolver(routeRepo)}
			request := GatewayPreflightRequest{SchemaVersion: 2, Model: "public-alias", Protocol: "responses"}
			group := &Group{ID: 42, Platform: PlatformComposite}
			result, err := gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
			require.NoError(t, err)
			require.Equal(t, "restricted", result.Routing.State)
			require.Equal(t, "OPERATOR_RESTRICTED", result.Routing.Reason)

			allowed := "channel-target"
			if source == BillingModelSourceUpstream {
				allowed = "account-target"
			}
			channel.ModelPricing[0].Models = []string{allowed}
			cache = populateChannelCache([]Channel{channel}, map[int64]string{42: PlatformComposite})
			cache.loadedAt = time.Now()
			channelService.cache.Store(cache)
			result, err = gateway.PreflightGatewayRequest(context.Background(), group, request, nil)
			require.NoError(t, err)
			require.Equal(t, "configured", result.Routing.State, "Composite then channel then account mapping must reach the configured upstream")
			require.Equal(t, "supported", result.Support.State)
		})
	}
}
