package service

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSonnet55RejectsUnsupportedParametersBeforeMimicry(t *testing.T) {
	fields := []string{
		`"thinking":{"type":"disabled"}`,
		`"thinking":{"type":"enabled","budget_tokens":1024}`,
		`"thinking":{"type":"between_tools"},"output_config":{"effort":"xhigh"}`,
		`"thinking":{"type":"between_tools","display":"summarized"}`,
		`"tool_choice":{"type":"any"}`,
		`"tool_choice":{"type":"tool","name":"lookup"}`,
		`"temperature":0.7`,
		`"top_p":0.5`,
		`"top_k":1`,
	}
	for _, field := range fields {
		for _, typ := range []string{AccountTypeOAuth, AccountTypeAPIKey, AccountTypeBedrock, AccountTypeServiceAccount} {
			for _, count := range []bool{false, true} {
				if count && (field == `"temperature":0.7` || field == `"top_p":0.5` || field == `"top_k":1`) {
					continue
				}
				model := "claude-sonnet-5-5"
				account := &Account{ID: 1, Platform: PlatformAnthropic, Type: typ}
				switch typ {
				case AccountTypeAPIKey:
					model = "public-sonnet"
					account.Credentials = map[string]any{"model_mapping": map[string]any{model: "claude-sonnet-5-5"}}
				case AccountTypeBedrock:
					account.Credentials = map[string]any{"aws_region": "eu-west-1"}
				case AccountTypeServiceAccount:
					model = "vertex-public-sonnet"
					account.Credentials = map[string]any{"model_mapping": map[string]any{model: "claude-sonnet-5-5@20260928"}}
				}
				body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hello"}],` + field + `}`)
				parsed := &ParsedRequest{Model: model, Body: NewRequestBodyRef(body)}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				svc := &GatewayService{}
				var err error
				if count {
					err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
				} else {
					_, err = svc.Forward(context.Background(), c, account, parsed)
				}
				if count && typ == AccountTypeBedrock {
					require.NoError(t, err)
					require.Equal(t, http.StatusNotFound, rec.Code)
					continue
				}
				require.Error(t, err, field)
				require.Equal(t, http.StatusBadRequest, rec.Code, field)
				require.Contains(t, rec.Body.String(), "invalid_request_error", field)
			}
		}
	}
}

func TestSonnet55PreservesSignedHistoryAndAcceptsDefaultParameters(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-5-5","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed"},{"type":"redacted_thinking","data":"encrypted"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}],"thinking":{"type":"between_tools"},"output_config":{"effort":"high"},"tool_choice":{"type":"none"},"temperature":1,"top_p":0.99}`)
	require.NoError(t, validateClaude55Request(body, "anthropic/claude-sonnet-5.5"))
	require.JSONEq(t, string(body), string(FilterThinkingBlocks(body, "claude-sonnet-5-5")))
	withoutThinking, _ := deleteJSONPathBytes(body, "thinking")
	require.JSONEq(t, string(withoutThinking), string(FilterThinkingBlocks(withoutThinking, "claude-sonnet-5-5")))
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-sonnet-5-5", claudeOAuthNormalizeOptions{})
	require.Equal(t, "none", gjson.GetBytes(out, "tool_choice.type").String())
	require.JSONEq(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(out, "messages").Raw)
}

func TestOpus55OpenRouterAliasPreservesSignedHistory(t *testing.T) {
	body := []byte(`{"model":"anthropic/claude-opus-5.5","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed"},{"type":"text","text":"answer"}]}],"thinking":{"type":"adaptive"},"output_config":{"effort":"xhigh"}}`)
	model := "anthropic/claude-opus-5.5"
	require.NoError(t, validateClaude55Request(body, model))
	require.JSONEq(t, string(body), string(FilterThinkingBlocks(body, model)))
	out, _ := normalizeClaudeOAuthRequestBody(body, model, claudeOAuthNormalizeOptions{})
	require.JSONEq(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(out, "messages").Raw)
	require.Equal(t, "xhigh", gjson.GetBytes(out, "output_config.effort").String())
}

func TestSonnet55CountTokensIgnoresGenerationSampling(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-5-5","messages":[{"role":"user","content":"hello"}],"temperature":0.7,"top_p":0.5,"top_k":1,"max_tokens":10}`)
	countBody := sanitizeCountTokensRequestBody(body)
	require.NoError(t, validateClaude55Request(countBody, "claude-sonnet-5-5"))
	require.Error(t, validateClaude55Request(body, "claude-sonnet-5-5"))
	for _, field := range []string{"temperature", "top_p", "top_k", "max_tokens"} {
		require.False(t, gjson.GetBytes(countBody, field).Exists())
	}
}

func TestSonnet55BedrockCCCompatTransformsBeforeValidation(t *testing.T) {
	groupID := int64(55)
	channels := &ChannelService{}
	channels.cache.Store(&channelCache{
		loadedAt: time.Now(),
		channelByGroupID: map[int64]*Channel{
			groupID: {Status: StatusActive, FeaturesConfig: map[string]any{featureKeyBedrockCCCompat: true}},
		},
	})
	svc := &GatewayService{channelService: channels}
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock,
		Credentials: map[string]any{"aws_region": "eu-west-1"}}
	for _, tc := range []struct {
		name         string
		thinking     string
		expectedType string
		wantErr      bool
	}{
		{name: "enabled", thinking: `{"type":"enabled","budget_tokens":2048}`, expectedType: "adaptive"},
		{name: "disabled", thinking: `{"type":"disabled"}`, expectedType: "between_tools"},
		{name: "between_tools invalid effort", thinking: `{"type":"between_tools"}`, expectedType: "between_tools", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"model":"claude-sonnet-5-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}],"thinking":` + tc.thinking + `,"output_config":{"effort":"low"}}`)
			if tc.wantErr {
				body = []byte(`{"model":"claude-sonnet-5-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}],"thinking":` + tc.thinking + `,"output_config":{"effort":"xhigh"}}`)
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			converted := svc.ApplyBedrockCCCompat(c, body, "claude-sonnet-5-5", account, &groupID)
			require.Equal(t, tc.expectedType, gjson.GetBytes(converted, "thinking.type").String())
			if tc.name == "enabled" {
				require.False(t, gjson.GetBytes(converted, "thinking.budget_tokens").Exists())
			}
			mapped, ok := ResolveBedrockModelID(account, "claude-sonnet-5-5")
			require.True(t, ok)
			err := validateClaude55Request(converted, mapped)
			if tc.wantErr {
				require.ErrorContains(t, err, "only low, medium or high effort")
				return
			}
			require.NoError(t, err)
			prepared, err := PrepareBedrockRequestBodyWithTokens(converted, mapped, nil, false)
			require.NoError(t, err)
			require.Equal(t, "low", gjson.GetBytes(prepared, "output_config.effort").String())
		})
	}
}
