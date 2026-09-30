package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIFastPolicyMissingTier(test *testing.T) {
	for _, scenario := range []struct {
		name, matcher, action, platform, accountType, model, want string
		userID                                                    int64
	}{
		{"oauth owner", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeOAuth, "gpt-5.5", "priority", 42},
		{"api key owner", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "priority", 42},
		{"other caller", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "", 43},
		{"unauthenticated", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "", 0},
		{"other model", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "custom-model", "", 42},
		{"other provider", OpenAIFastTierMissing, OpenAIFastPolicyActionForcePriority, PlatformGemini, AccountTypeAPIKey, "gpt-5.5", "", 42},
		{"legacy all", OpenAIFastTierAny, OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "", 42},
		{"legacy empty", "", OpenAIFastPolicyActionForcePriority, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "", 42},
		{"non-force action", OpenAIFastTierMissing, BetaPolicyActionBlock, PlatformOpenAI, AccountTypeAPIKey, "gpt-5.5", "", 42},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
				ServiceTier: scenario.matcher, Action: scenario.action, Scope: BetaPolicyScopeAll,
				UserIDs: []int64{42}, ModelWhitelist: []string{"gpt-5.5"}, FallbackAction: BetaPolicyActionPass,
			}}}
			svc := newOpenAIGatewayServiceWithSettings(test, settings)
			ctx := context.WithValue(context.Background(), ctxkey.UserID, scenario.userID)
			account := &Account{Platform: scenario.platform, Type: scenario.accountType}
			body := []byte(`{"model":"` + scenario.model + `","input":"hello"}`)
			updated, err := svc.applyOpenAIFastPolicyToBody(ctx, account, scenario.model, body)
			require.NoError(test, err)
			require.Equal(test, scenario.want, gjson.GetBytes(updated, "service_tier").String())
			frame := []byte(`{"type":"response.create","model":"` + scenario.model + `","input":"hello"}`)
			updated, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, scenario.model, frame)
			require.NoError(test, err)
			require.Nil(test, blocked)
			require.Equal(test, scenario.want, gjson.GetBytes(updated, "service_tier").String())
			otherFrame := []byte(`{"type":"response.cancel"}`)
			updated, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, scenario.model, otherFrame)
			require.NoError(test, err)
			require.Nil(test, blocked)
			require.Equal(test, otherFrame, updated)
		})
	}
}

func TestForwardMissingTierPreservesBillingContext(test *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct {
		userID int64
		tier   string
		want   string
	}{{42, "", "priority"}, {43, "", ""}, {42, "flex", "flex"}} {
		svc := newOpenAIGatewayServiceWithSettings(test, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierMissing, Action: OpenAIFastPolicyActionForcePriority, Scope: BetaPolicyScopeAll, UserIDs: []int64{42},
		}}})
		svc.cfg = &config.Config{}
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"resp_1","object":"response","status":"completed","model":"gpt-5.5","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
		}}
		svc.httpUpstream = upstream
		body := []byte(`{"model":"gpt-5.5","input":"hello","stream":false}`)
		if scenario.tier != "" {
			body = []byte(`{"model":"gpt-5.5","input":"hello","stream":false,"service_tier":"` + scenario.tier + `"}`)
		}
		ctx := context.WithValue(context.Background(), ctxkey.UserID, scenario.userID)
		ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
		ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
		account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
			Credentials: map[string]any{"api_key": "sk-test"}, Extra: map[string]any{"openai_responses_supported": true}, Status: StatusActive, Schedulable: true}
		result, err := svc.Forward(ctx, ginContext, account, body)
		require.NoError(test, err)
		if scenario.want != "" {
			require.Equal(test, scenario.want, gjson.GetBytes(upstream.lastBody, "service_tier").String())
			require.NotNil(test, result.ServiceTier)
			require.Equal(test, scenario.want, *result.ServiceTier)
		} else {
			require.False(test, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
			require.Nil(test, result.ServiceTier)
		}
	}
}

func TestMissingTierPolicySettingsRoundTrip(test *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(test, nil)
	settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
		ServiceTier: OpenAIFastTierMissing, Action: OpenAIFastPolicyActionForcePriority, Scope: BetaPolicyScopeAll,
	}}}
	require.NoError(test, svc.settingService.SetOpenAIFastPolicySettings(context.Background(), settings))
	stored, err := svc.settingService.GetOpenAIFastPolicySettings(context.Background())
	require.NoError(test, err)
	require.Equal(test, settings, stored)
	_, err = ValidateOpenAIServiceTierField([]byte(`{"service_tier":"missing"}`))
	require.Error(test, err)
}
