package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type compositeWSRouteRepo struct {
	service.CompositeModelRouteRepository
	routes []service.CompositeModelRoute
	err    error
}

func (r *compositeWSRouteRepo) ListByGroup(context.Context, int64, bool) ([]service.CompositeModelRoute, error) {
	return r.routes, r.err
}

type compositeWSHTTPUpstream struct{ service.HTTPUpstream }

func (*compositeWSHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func compositeWSResolver(platform, endpoint, upstream string) *service.CompositeRouteResolver {
	return service.NewCompositeRouteResolver(&compositeWSRouteRepo{routes: []service.CompositeModelRoute{{
		GroupID: 4201, PublicModel: "public-alias", MatchType: service.CompositeRouteMatchExact,
		TargetPlatform: platform, Endpoint: endpoint, UpstreamModel: upstream, Enabled: true,
	}}})
}
func compositeWSGroup(models ...string) *service.Group {
	g := wsAllowlistGroup(len(models) > 0, models...)
	g.Platform = service.PlatformComposite
	return g
}

func TestOpenAIResponsesWebSocket_CompositeAlias(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		for _, endpoint := range []string{service.CompositeRouteEndpointResponses, service.CompositeRouteEndpointAny} {
			for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
				t.Run(platform+"/"+endpoint+"/"+mode, func(t *testing.T) {
					upstream := "gpt-5.4"
					if platform == service.PlatformGrok {
						upstream = "grok-4.3"
					}
					got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
						firstPayload:  `{"type":"response.create","model":"public-alias","input":"hi"}`,
						secondPayload: `{"type":"response.create","input":"again"}`,
						group:         compositeWSGroup("public-alias"), accountPlatform: platform, ingressMode: mode,
						compositeResolver: compositeWSResolver(platform, endpoint, upstream),
					})
					require.Len(t, got.upstreamPayloads, 2)
					for i, payload := range got.upstreamPayloads {
						require.Equal(t, upstream, gjson.GetBytes(payload, "model").String())
						require.Equal(t, "public-alias", gjson.GetBytes(got.clientEvents[i], "response.model").String())
						require.Equal(t, "public-alias", got.logs[i].RequestedModel)
						require.NotNil(t, got.logs[i].UpstreamModel)
						require.Equal(t, upstream, *got.logs[i].UpstreamModel)
					}
				})
			}
		}
	}
}

func TestOpenAIResponsesWebSocket_CompositeRouteRejections(t *testing.T) {
	for _, tc := range []struct {
		name, platform, endpoint, model, reason string
		repoErr                                 error
		status                                  coderws.StatusCode
	}{
		{name: "disallowed platform", platform: service.PlatformAnthropic, endpoint: "responses", model: "public-alias", reason: "only supports OpenAI-compatible"},
		{name: "wrong endpoint", platform: service.PlatformOpenAI, endpoint: "messages", model: "public-alias", reason: "only supports OpenAI-compatible"},
		{name: "unknown alias", platform: service.PlatformOpenAI, endpoint: "responses", model: "unknown-alias", reason: "only supports OpenAI-compatible"},
		{name: "resolver error", model: "gpt-5.4", repoErr: errors.New("database unavailable"), reason: "Failed to resolve composite model route", status: coderws.StatusInternalError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := compositeWSResolver(tc.platform, tc.endpoint, "gpt-5.4")
			if tc.repoErr != nil {
				resolver = service.NewCompositeRouteResolver(&compositeWSRouteRepo{err: tc.repoErr})
			}
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload: `{"type":"response.create","model":"` + tc.model + `"}`, group: compositeWSGroup(), compositeResolver: resolver,
				firstFrameCloseExpected: true, closeReason: tc.reason, closeStatus: tc.status,
			})
		})
	}
}

func TestOpenAIResponsesWebSocket_CompositeAdmissionUsesPublicModel(t *testing.T) {
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"public-alias"}`, group: compositeWSGroup("gpt-5.4"),
		compositeResolver: compositeWSResolver(service.PlatformOpenAI, "responses", "gpt-5.4"), firstFrameCloseExpected: true,
	})
}

func TestOpenAIResponsesWebSocket_CompositeAdmissionPreservesRawCandidates(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
			t.Run(platform+"/"+mode, func(t *testing.T) {
				upstream := "gpt-5.4"
				if platform == service.PlatformGrok {
					upstream = "grok-4.3"
				}
				runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
					firstPayload:  `{"type":"response.create","model":"public-alias"}`,
					secondPayload: `{"type":"response.create","model":"public-alias","MODEL":"forbidden"}`,
					group:         compositeWSGroup("public-alias"), accountPlatform: platform, ingressMode: mode,
					compositeResolver:       compositeWSResolver(platform, "responses", upstream),
					secondTurnCloseExpected: true, closeReason: "model switch requires reconnect",
				})
			})
		}
	}
}

func TestOpenAIResponsesWebSocket_CompositeChannelAccountMappingOrder(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:  `{"type":"response.create","model":"public-alias"}`,
				secondPayload: `{"type":"response.create"}`, group: compositeWSGroup("public-alias"), ingressMode: mode,
				compositeResolver:   compositeWSResolver(service.PlatformOpenAI, "responses", "route-target"),
				channelMapping:      map[string]string{"route-target": "channel-target"},
				accountModelMapping: map[string]any{"channel-target": "gpt-5.4"},
			})
			for i, payload := range got.upstreamPayloads {
				require.Equal(t, "gpt-5.4", gjson.GetBytes(payload, "model").String())
				require.Equal(t, "public-alias", gjson.GetBytes(got.clientEvents[i], "response.model").String())
				require.Equal(t, "public-alias", got.logs[i].RequestedModel)
			}
		})
	}
}

func TestOpenAIResponsesWebSocket_CompositeModelSwitchRequiresReconnect(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		for _, model := range []string{"gpt-5.4", "grok-4.3", "unknown-alias"} {
			t.Run(mode+"/"+model, func(t *testing.T) {
				runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
					firstPayload:  `{"type":"response.create","model":"public-alias"}`,
					secondPayload: `{"type":"response.create","model":"` + model + `"}`, group: compositeWSGroup(), ingressMode: mode,
					compositeResolver:       compositeWSResolver(service.PlatformOpenAI, "responses", "gpt-5.4"),
					secondTurnCloseExpected: true, closeReason: "model switch requires reconnect",
				})
			})
		}
	}
}

func TestOpenAIResponsesWebSocket_CompositeChannelBilling(t *testing.T) {
	for _, source := range []string{service.BillingModelSourceRequested, service.BillingModelSourceChannelMapped, service.BillingModelSourceUpstream} {
		t.Run(source, func(t *testing.T) {
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:      `{"type":"response.create","model":"gpt-5.6-sol"}`,
				secondPayload:     `{"type":"response.create","model":"gpt-5.6-sol"}`,
				group:             compositeWSGroup("gpt-5.6-sol"),
				compositeResolver: service.NewCompositeRouteResolver(&compositeWSRouteRepo{routes: []service.CompositeModelRoute{{PublicModel: "gpt-5.6-sol", MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformOpenAI, Endpoint: service.CompositeRouteEndpointResponses, UpstreamModel: "route-target"}}}),
				channelMapping:    map[string]string{"route-target": "gpt-5.4"}, billingModelSource: source,
				accountModelMapping: map[string]any{"gpt-5.4": "gpt-5.4"},
			})
			for i, log := range got.logs {
				require.Equal(t, "gpt-5.4", gjson.GetBytes(got.upstreamPayloads[i], "model").String())
				require.Equal(t, "gpt-5.6-sol", log.RequestedModel)
				require.Equal(t, "gpt-5.6-sol", log.Model)
				if source == service.BillingModelSourceRequested {
					require.InDelta(t, 40e-6, log.TotalCost, 1e-12)
				} else {
					require.InDelta(t, 20e-6, log.TotalCost, 1e-12)
				}
			}
		})
	}
}

func TestOpenAIResponsesWebSocket_CompositeDetectorFallback(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"gpt-5.4"}`, group: compositeWSGroup(),
		compositeResolver: service.NewCompositeRouteResolver(&compositeWSRouteRepo{}),
	})
	require.Equal(t, "gpt-5.4", gjson.GetBytes(got.upstreamFirstPayload, "model").String())
}

func TestOpenAIResponsesWebSocket_CompositeSessionModelSwitchRequiresReconnect(t *testing.T) {
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:  `{"type":"response.create","model":"public-alias"}`,
		secondPayload: `{"type":"session.update","session":{"model":"grok-4.3"}}`, group: compositeWSGroup(),
		compositeResolver:       compositeWSResolver(service.PlatformOpenAI, "responses", "gpt-5.4"),
		secondTurnCloseExpected: true, closeReason: "model switch requires reconnect",
	})
}

func TestOpenAIResponsesWebSocket_CompositeSessionModelRepeatPreservesIdentity(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:  `{"type":"response.create","model":"public-alias"}`,
		midPayload:    `{"type":"session.update","session":{"model":"public-alias"}}`,
		secondPayload: `{"type":"response.create"}`, group: compositeWSGroup(),
		compositeResolver: compositeWSResolver(service.PlatformOpenAI, "responses", "gpt-5.4"),
	})
	require.Equal(t, "gpt-5.4", gjson.GetBytes(got.upstreamPayloads[1], "session.model").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(got.upstreamPayloads[2], "model").String())
	require.Equal(t, "public-alias", got.logs[2].RequestedModel)
}

func TestCompositeWSModelRejectionDoesNotReportAccountFailure(t *testing.T) {
	err := service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "model switch requires reconnect", nil)
	require.False(t, shouldReportOpenAIWSProxyAccountFailure(err))
}

func TestGPT61CompositeWebSocketFinalEffort(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		for _, modelField := range []string{`"model":"public-alias",`, ""} {
			t.Run(mode+"/"+modelField, func(t *testing.T) {
				for _, effort := range []string{"none", "minimal", "ultra"} {
					runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
						firstPayload:  `{"type":"response.create","model":"public-alias","input":"hi"}`,
						secondPayload: `{"type":"response.create",` + modelField + `"reasoning":{"effort":"` + effort + `"}}`,
						group:         compositeWSGroup("public-alias"), ingressMode: mode,
						compositeResolver:       compositeWSResolver(service.PlatformOpenAI, "responses", "route-target"),
						channelMapping:          map[string]string{"route-target": "channel-target"},
						accountModelMapping:     map[string]any{"channel-target": "gpt-6.1-sol"},
						secondTurnCloseExpected: true,
						closeReason:             "gpt-6.1-sol does not support reasoning effort",
					})
				}
				got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
					firstPayload:  `{"type":"response.create","model":"public-alias","input":"hi"}`,
					secondPayload: `{"type":"response.create",` + modelField + `"input":"again"}`,
					group:         compositeWSGroup("public-alias"), ingressMode: mode,
					compositeResolver:   compositeWSResolver(service.PlatformOpenAI, "responses", "route-target"),
					channelMapping:      map[string]string{"route-target": "channel-target"},
					accountModelMapping: map[string]any{"channel-target": "gpt-6.1-sol-max"},
				})
				for _, payload := range got.upstreamPayloads {
					require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(payload, "model").String())
					require.Equal(t, "max", gjson.GetBytes(payload, "reasoning.effort").String())
				}
			})
		}
	}
}

func TestGPT61CompositeWebSocketPublicModelEffortPolicy(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		for _, first := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/first=%v", mode, first), func(t *testing.T) {
				group := compositeWSGroup("public-alias")
				group.ReasoningEffortMappings = []service.ReasoningEffortMapping{{From: "high", To: "deny", MatchType: "exact", Model: "public-alias"}}
				firstPayload := `{"type":"response.create","model":"public-alias","input":"hi"}`
				if first {
					firstPayload = `{"type":"response.create","model":"public-alias","reasoning":{"effort":"high"}}`
				}
				runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
					firstPayload:  firstPayload,
					secondPayload: `{"type":"response.create","model":"public-alias","reasoning":{"effort":"high"}}`,
					group:         group, ingressMode: mode,
					compositeResolver:       compositeWSResolver(service.PlatformOpenAI, "responses", "gpt-6.1-sol"),
					firstFrameCloseExpected: first, secondTurnCloseExpected: !first,
					closeReason: "reasoning effort",
				})
			})
		}
	}
}
