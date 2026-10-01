//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFinalNativeOpusAliasUsesMappedProtocol(t *testing.T) {
	for _, chat := range []bool{false, true} {
		for _, forced := range []bool{false, true} {
			body := `{"model":"public-opus","input":"hello","reasoning":{"effort":"xhigh"}}`
			if chat {
				body = `{"model":"public-opus","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"xhigh"}`
			}
			if forced {
				body = body[:len(body)-1] + `,"tool_choice":"required","tools":[{"type":"function","name":"lookup","function":{"name":"lookup"}}]}`
			}
			upstream := &httpUpstreamRecorder{resp: nativeAnthropicStreamResponse()}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			account := nativeAnthropicTestAccount()
			account.Credentials["model_mapping"] = map[string]any{"public-opus": "claude-opus-5-5"}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
			var err error
			if chat {
				_, err = svc.forwardChatCompletionsViaNativeAnthropic(context.Background(), ctx, account, []byte(body), "")
			} else {
				_, err = svc.forwardResponsesViaNativeAnthropic(context.Background(), ctx, account, []byte(body), "")
			}
			if forced {
				require.Error(t, err)
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Nil(t, upstream.lastReq)
			} else {
				require.NoError(t, err)
				require.Equal(t, "claude-opus-5-5", gjson.GetBytes(upstream.lastBody, "model").String())
				require.Equal(t, "adaptive", gjson.GetBytes(upstream.lastBody, "thinking.type").String())
				require.Equal(t, "xhigh", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
				require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
			}
		}
	}
}

func TestFinalNativeOpusSignedThinking(t *testing.T) {
	payload := strings.Join([]string{
		"event: message_start\n" + `data: {"type":"message_start","message":{"id":"msg_1","model":"claude-opus-5-5","content":[],"usage":{"input_tokens":10}}}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"signed"}}`,
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"ok"}}`,
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":1}`,
		"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		"event: message_stop\n" + `data: {"type":"message_stop"}`,
	}, "\n\n") + "\n\n"
	for _, stream := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		response := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(payload))}
		svc := &OpenAIGatewayService{}
		var err error
		if stream {
			_, err = svc.handleResponsesStreamingFromNativeAnthropic(response, ctx, "public-opus", "claude-opus-5-5", "claude-opus-5-5", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
		} else {
			_, err = svc.handleResponsesBufferedFromNativeAnthropic(response, ctx, "public-opus", "claude-opus-5-5", "claude-opus-5-5", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
		}
		require.NoError(t, err)
		require.Contains(t, recorder.Body.String(), "anthropic-thinking-v1:")
		require.Contains(t, recorder.Body.String(), "public-opus")
		require.Contains(t, recorder.Body.String(), `"text":"ok"`)
	}
}

func TestFinalOpusGuardBeforeMimicry(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		for _, field := range []string{`"thinking":{"type":"disabled"}`, `"thinking":{"type":"enabled","budget_tokens":1024}`, `"tool_choice":{"type":"any"}`, `"tool_choice":{"type":"tool","name":"lookup"}`} {
			for _, count := range []bool{false, true} {
				model := "claude-opus-5-5"
				account := &Account{ID: 1, Platform: PlatformAnthropic, Type: accountType}
				if accountType == AccountTypeAPIKey {
					model = "public-opus"
					account.Credentials = map[string]any{"model_mapping": map[string]any{model: "claude-opus-5-5"}}
				}
				body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hello"}],` + field + `}`)
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
				parsed := &ParsedRequest{Model: model, Body: NewRequestBodyRef(body)}
				svc := &GatewayService{}
				var err error
				if count {
					err = svc.ForwardCountTokens(context.Background(), ctx, account, parsed)
				} else {
					_, err = svc.Forward(context.Background(), ctx, account, parsed)
				}
				require.Error(t, err)
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Contains(t, recorder.Body.String(), "invalid_request_error")
			}
		}
	}
}

func TestFinalOpusOAuthKeepsDefaultsAndToolChoice(t *testing.T) {
	body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hello"}],"tool_choice":{"type":"none"}}`)
	output, _ := normalizeClaudeOAuthRequestBody(body, "claude-opus-5-5", claudeOAuthNormalizeOptions{})
	require.False(t, gjson.GetBytes(output, "temperature").Exists())
	require.Equal(t, "none", gjson.GetBytes(output, "tool_choice.type").String())
}

func TestFinalOpusPricingFamilyDoesNotMatch55(t *testing.T) {
	svc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"claude-opus-5-5": {InputCostPerToken: 4e-6, OutputCostPerToken: 20e-6},
	}}
	require.Nil(t, svc.matchByModelFamily("claude-opus-5"))
}

func TestFinalGPT6ExplicitZeroRemoteCacheWrite(t *testing.T) {
	svc := &PricingService{}
	var err error
	svc.pricingData, err = svc.parsePricingData([]byte(`{"gpt-6-sol":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"input_cost_per_token_flex":0.000001,"cache_creation_input_token_cost":0},"gpt-6-luna":{"litellm_provider":"openai","input_cost_per_token":0.0000001,"output_cost_per_token":0.0000005,"input_cost_per_token_flex":0.00000005,"cache_creation_input_token_cost":0}}`))
	require.NoError(t, err)
	billing := NewBillingService(rawChatCompletionsTestConfig(), svc)
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna"} {
		for _, tier := range []string{"", "priority", "flex"} {
			cost, err := billing.CalculateCostWithServiceTier(model, UsageTokens{CacheCreationTokens: 1000}, 1, tier)
			require.NoError(t, err)
			require.Zero(t, cost.CacheCreationCost)
		}
	}
}
