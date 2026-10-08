package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT61SolMetadataAndCompatibility(t *testing.T) {
	encoded, err := json.Marshal(newConfiguredCodexModelDescriptor("gpt-6.1-sol"))
	require.NoError(t, err)
	var descriptor map[string]any
	require.NoError(t, json.Unmarshal(encoded, &descriptor))
	require.Equal(t, "medium", descriptor["default_reasoning_level"])
	require.Equal(t, true, descriptor["prefer_websockets"])
	levels := descriptor["supported_reasoning_levels"].([]any)
	require.Len(t, levels, 5)
	require.Equal(t, "max", levels[4].(map[string]any)["effort"])
	require.Contains(t, descriptor["model_messages"].(map[string]any), "tools")
	for _, model := range []string{"gpt-6.1-sol", "openai/gpt-6.1-sol-max", "GPT_6.1_SOL"} {
		require.Equal(t, "gpt-6.1-sol", normalizeKnownOpenAICodexModel(model))
		require.Equal(t, "gpt-6.1-sol", normalizeOpenAIModelForUpstream(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, model))
		require.True(t, shouldAutoInjectPromptCacheKeyForCompat(model))
	}
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		req := &apicompat.AnthropicRequest{Model: "openai/gpt-6.1-sol-" + effort}
		applyOpenAICompatModelNormalization(req)
		require.Equal(t, "gpt-6.1-sol", req.Model)
		require.Equal(t, effort, req.OutputConfig.Effort)
		require.Equal(t, effort, *extractOpenAIReasoningEffortFromBody(nil, "gpt-6.1-sol", "openai/gpt-6.1-sol-"+effort))
	}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://proxy.example/v1", "model_mapping": map[string]any{"public": "gpt-6.1-sol"}}}
	for _, body := range []string{`{"model":"public","reasoning":{"effort":"none"}}`, `{"model":"public","reasoning":{"effort":"ultra"}}`, `{"model":"public","reasoning":{"effort":false}}`} {
		_, err := filterOpenAIResponsesNoneReasoningEffortForAccount(account, []byte(body))
		require.Error(t, err)
		_, _, err = normalizeOpenAIResponsesWebSocketCompatibilityBody([]byte(body), account, false)
		require.Error(t, err)
	}
}

func TestGPT61SolNativeForwardFinalMapping(t *testing.T) {
	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
		for _, compact := range []bool{false, true} {
			for _, effort := range []string{"low", "medium", "high", "xhigh", "max", "none", "minimal"} {
				cfg := &config.Config{}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture stops after capture"}}`))}}
				svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: accountType, Concurrency: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "fixture-key", "access_token": "fixture-token", "chatgpt_account_id": "fixture-account", "model_mapping": map[string]any{"public": "openai/gpt-6.1-sol-" + effort}}}
				path := "/v1/responses"
				if compact {
					path += "/compact"
					account.Credentials["model_mapping"] = map[string]any{"public": "gpt-5.4"}
					account.Credentials["compact_model_mapping"] = map[string]any{"public": "openai/gpt-6.1-sol-" + effort}
				}
				body := []byte(`{"model":"public","input":"hi","instructions":"fixture","stream":true}`)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
				_, err := svc.Forward(context.Background(), c, account, body)
				require.Error(t, err)
				if effort == "none" || effort == "minimal" {
					require.Nil(t, upstream.lastReq)
					require.Contains(t, err.Error(), "reasoning effort")
				} else {
					require.NotNil(t, upstream.lastReq)
					require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(upstream.lastBody, "model").String(), accountType, compact, effort)
					require.Equal(t, effort, gjson.GetBytes(upstream.lastBody, "reasoning.effort").String(), accountType, compact, effort)
				}
			}
		}
	}
}

func TestGPT61SolNativeWebSocketMappedAlias(t *testing.T) {
	for _, effort := range []string{"max", "none", "minimal"} {
		t.Run(effort, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Gateway.OpenAIWS.Enabled = true
			cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
			cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
			cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
			cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
			cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
			capture := &openAIWSCaptureConn{events: [][]byte{[]byte(`{"type":"response.completed","response":{"id":"resp_fixture","model":"gpt-6.1-sol","usage":{"input_tokens":1,"output_tokens":1}}}`)}}
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(&openAIWSCaptureDialer{conn: capture})
			svc := &OpenAIGatewayService{cfg: cfg, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), openaiWSPool: pool}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{"responses_websockets_v2_enabled": true}, Credentials: map[string]any{"api_key": "fixture", "model_mapping": map[string]any{"public": "openai/gpt-6.1-sol-" + effort}}}
			serverErrors := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					serverErrors <- err
					return
				}
				defer conn.CloseNow()
				_, first, err := conn.Read(r.Context())
				if err != nil {
					serverErrors <- err
					return
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = r
				serverErrors <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "fixture", first, nil)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			require.NoError(t, err)
			defer client.CloseNow()
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"public","input":"hi","stream":false}`)))
			_, _, readErr := client.Read(ctx)
			if effort == "max" {
				require.NoError(t, readErr)
				_ = client.Close(coderws.StatusNormalClosure, "done")
			} else {
				require.Error(t, readErr)
			}
			select {
			case serverErr := <-serverErrors:
				if effort == "max" {
					require.NoError(t, serverErr)
				} else {
					require.Error(t, serverErr)
				}
			case <-ctx.Done():
				t.Fatal("websocket fixture timed out")
			}
			if effort == "max" {
				require.Len(t, capture.writes, 1)
				require.Equal(t, "gpt-6.1-sol", gjson.Get(requestToJSONString(capture.writes[0]), "model").String())
				require.Equal(t, "max", gjson.Get(requestToJSONString(capture.writes[0]), "reasoning.effort").String())
			} else {
				require.Empty(t, capture.writes)
			}
		})
	}
}

func TestGPT61SolCompactConfiguredFallbackAndPassthrough(t *testing.T) {
	for _, effort := range []string{"max", "none", "minimal"} {
		for _, passthrough := range []bool{false, true} {
			cfg := &config.Config{}
			cfg.Gateway.OpenAICompactModel = "gpt-6.1-sol-" + effort
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture"}}`))}}
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture"}}
			if passthrough {
				account.Extra = map[string]any{"openai_passthrough": true}
			}
			body := []byte(`{"model":"public","input":"hi"}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
			_, err := svc.Forward(context.Background(), c, account, body)
			require.Error(t, err)
			if effort == "max" {
				require.NotNil(t, upstream.lastReq)
				require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(upstream.lastBody, "model").String())
				require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
			} else {
				require.Nil(t, upstream.lastReq)
				require.Contains(t, err.Error(), "reasoning effort")
			}
		}
	}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_passthrough": true}, Credentials: map[string]any{"base_url": "https://proxy.example", "model_mapping": map[string]any{"public": "gpt-6.1-sol"}}}
	body := []byte(`{"model":"public","reasoning":{"effort":"none"}}`)
	_, err := filterOpenAIResponsesNoneReasoningEffortForAccount(account, body)
	require.NoError(t, err)
	out, _, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(body, account, false)
	require.NoError(t, err)
	require.Equal(t, "public", gjson.GetBytes(out, "model").String())
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		account := &Account{Platform: PlatformOpenAI, Type: accountType, Credentials: map[string]any{"model_mapping": map[string]any{"public": "openai/gpt-6.1-sol-max"}}}
		out, _, err := normalizeOpenAIResponsesWebSocketCompatibilityBody([]byte(`{"type":"response.create","model":"public","input":"hi"}`), account, false)
		require.NoError(t, err)
		require.Equal(t, "max", gjson.GetBytes(out, "reasoning.effort").String())
	}
}

func TestGPT61SolAuthenticationOnlyPassthroughActualModel(t *testing.T) {
	for _, model := range []string{"public", "openai/gpt-6.1-sol-max"} {
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture"}}`))}}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_passthrough": true}, Credentials: map[string]any{"api_key": "fixture", "model_mapping": map[string]any{model: "gpt-6.1-sol-none"}}}
		body := []byte(`{"model":"` + model + `","input":"hi"}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		_, err := svc.Forward(context.Background(), c, account, body)
		require.Error(t, err)
		require.NotNil(t, upstream.lastReq)
		require.Equal(t, model, gjson.GetBytes(upstream.lastBody, "model").String())
		require.False(t, gjson.GetBytes(upstream.lastBody, "reasoning.effort").Exists())
	}
}

func TestGPT61SolFinalMappingPreservesOtherModelCompactRules(t *testing.T) {
	fallbackConfig := &config.Config{}
	fallbackConfig.Gateway.OpenAICompactModel = "gpt-5.4-high"
	fallbackService := &OpenAIGatewayService{cfg: fallbackConfig}
	require.Equal(t, "gpt-5.4", fallbackService.resolveOpenAICompactFallbackModel(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "public"))
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": map[string]any{"public": "gpt-5.4-high"}}}
	require.Equal(t, "gpt-5.4", resolveOpenAIAccountUpstreamModelForRequest(account, "public", true))
	account.Credentials["compact_model_mapping"] = map[string]any{"public": "gpt-5.4-openai-compact"}
	require.Equal(t, "gpt-5.4-openai-compact", resolveOpenAIAccountUpstreamModelForRequest(account, "public", true))

	account = &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "fixture-key", "base_url": "https://api.openai.com", "model_mapping": map[string]any{"public": "gpt-5.4"}, "compact_model_mapping": map[string]any{"public": "gpt-6.1-sol-none"}},
		Extra:       map[string]any{"openai_responses_mode": "force_chat_completions"}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAICompactModel = "gpt-6.1-sol-minimal"
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture"}}`))}}
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	body := []byte(`{"model":"public","input":"hi"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestGPT61SolRejectsUnsupportedAliasAndDisablesAPIKeyLite(t *testing.T) {
	for _, model := range []string{"gpt-6.1-sol-ultra", "openai/gpt-6.1-sol-ultra"} {
		require.Error(t, validateGPT61SolCompatRequest([]byte(`{"model":"public"}`), model))
		require.Error(t, validateGPT61SolCompatRequest([]byte(`{"model":"`+model+`"}`), "gpt-6.1-sol"))
	}
	for _, model := range []string{"gpt-6.1-sol-max", "openai/gpt-6.1-sol-high"} {
		body, err := adjustAPIKeyCodexModelsManifest([]byte(`{"models":[{"slug":"`+model+`","use_responses_lite":true}]}`), nil)
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(body, "models.0.use_responses_lite").Bool())
		body, err = adjustAPIKeyCodexModelsManifest([]byte(`{"models":[{"slug":"public","use_responses_lite":true}]}`), &Account{Credentials: map[string]any{"model_mapping": map[string]any{"public": model}}})
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(body, "models.0.use_responses_lite").Bool())
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.openai.com"}}
		account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: map[string]UpstreamModelMetadata{model: {CodexToolCapabilities: map[string]json.RawMessage{"use_responses_lite": json.RawMessage("true")}}}})
		require.JSONEq(t, "false", string(accountCodexToolCapabilities(account, model)["use_responses_lite"]))
	}
}

func TestGPT61SolPricingTiersAndThreshold(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)
	for _, contextTokens := range []int{272000, 272001} {
		inputMultiplier, outputMultiplier := 1.0, 1.0
		if contextTokens > 272000 {
			inputMultiplier, outputMultiplier = 2, 1.5
		}
		for tier, multiplier := range map[string]float64{"": 1, "priority": 2, "flex": 0.5} {
			cost, err := svc.CalculateCostWithServiceTier("openai/gpt-6.1-sol-max", UsageTokens{InputTokens: contextTokens - 1100, CacheReadTokens: 100, CacheCreationTokens: 1000, OutputTokens: 100}, 1, tier)
			require.NoError(t, err)
			require.InDelta(t, float64(contextTokens-1100)*2e-6*inputMultiplier*multiplier, cost.InputCost, 1e-10)
			require.InDelta(t, 100*0.1e-6*inputMultiplier*multiplier, cost.CacheReadCost, 1e-10)
			require.InDelta(t, 1000*2.5e-6*inputMultiplier*multiplier, cost.CacheCreationCost, 1e-10)
			require.InDelta(t, 100*10e-6*outputMultiplier*multiplier, cost.OutputCost, 1e-10)
		}
	}
	pricing := &PricingService{}
	var err error
	pricing.pricingData, err = pricing.parsePricingData([]byte(`{"gpt-6.1-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_creation_input_token_cost":0,"cache_creation_input_token_cost_priority":0.000005}}`))
	require.NoError(t, err)
	svc = NewBillingService(&config.Config{}, pricing)
	for _, tier := range []string{"", "priority", "flex"} {
		cost, err := svc.CalculateCostWithServiceTier("gpt-6.1-sol", UsageTokens{CacheCreationTokens: 300000}, 1, tier)
		require.NoError(t, err)
		require.Zero(t, cost.CacheCreationCost)
	}
}
