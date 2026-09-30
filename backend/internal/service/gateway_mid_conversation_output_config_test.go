//go:build unit

package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_MultiMessage(t *testing.T) {
	body := []byte(`{
		"model":"claude-opus-5",
		"output_config":{"effort":"high"},
		"messages":[
			{"role":"system","content":[],"output_config":{"effort":"high"}},
			{"role":"user","content":"hello"},
			{"role":"system","content":"You are helpful.","output_config":{"effort":"high"},"cache_control":{"type":"ephemeral"}},
			{"role":"assistant","content":[{"type":"text","text":"hi"}],"output_config":{"effort":"high"}},
			{"role":"system","content":[{"type":"text","text":""}],"output_config":{"effort":"high"}},
			{"role":"system","content":null,"output_config":{"effort":"high"}},
			{"role":"system","content":"","output_config":{"effort":"high"}},
			{"role":"system","output_config":{"effort":"high"}}
		]
	}`)

	out, changed := sanitizeAnthropicBodyForBetaTokens(body, "claude-code-20250219,oauth-2025-04-20")
	require.True(t, changed, "存在 message-level output_config 且缺 beta → 必须发生净化")

	msgs := gjson.GetBytes(out, "messages").Array()
	require.Len(t, msgs, 3, "空控制 system 整条删除；仅保留 user + 有正文 system + assistant")

	require.Equal(t, "user", gjson.GetBytes(out, "messages.0.role").String())
	require.Equal(t, "hello", gjson.GetBytes(out, "messages.0.content").String(), "用户消息顺序不得改变")

	require.Equal(t, "system", gjson.GetBytes(out, "messages.1.role").String())
	require.Equal(t, "You are helpful.", gjson.GetBytes(out, "messages.1.content").String())
	require.False(t, gjson.GetBytes(out, "messages.1.output_config").Exists(),
		"有正文 system 只剥 output_config，不整条删除")
	require.True(t, gjson.GetBytes(out, "messages.1.cache_control").Exists(),
		"system 其余字段必须保留")

	require.Equal(t, "assistant", gjson.GetBytes(out, "messages.2.role").String())
	require.Equal(t, "hi", gjson.GetBytes(out, "messages.2.content.0.text").String())
	require.False(t, gjson.GetBytes(out, "messages.2.output_config").Exists(),
		"user/assistant 只剥字段，绝不整条删除")

	require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
	require.Equal(t, "claude-opus-5", gjson.GetBytes(out, "model").String())
}

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_EmptySystemVariants(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"missing_content", `{"role":"system","output_config":{"effort":"high"}}`},
		{"null_content", `{"role":"system","content":null,"output_config":{"effort":"high"}}`},
		{"empty_string_content", `{"role":"system","content":"","output_config":{"effort":"high"}}`},
		{"empty_array_content", `{"role":"system","content":[],"output_config":{"effort":"high"}}`},
		{"only_empty_text_block", `{"role":"system","content":[{"type":"text","text":""}],"output_config":{"effort":"high"}}`},
		{"only_empty_text_blocks", `{"role":"system","content":[{"type":"text","text":""},{"type":"text","text":""}],"output_config":{"effort":"high"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"messages":[` + tc.msg + `,{"role":"user","content":"hi"}],"output_config":{"effort":"high"}}`)
			out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
			require.True(t, changed)

			msgs := gjson.GetBytes(out, "messages").Array()
			require.Len(t, msgs, 1, "无正文的 system 控制消息必须整条删除")
			require.Equal(t, "user", msgs[0].Get("role").String())
			require.Equal(t, "hi", msgs[0].Get("content").String())
			require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String(),
				"顶层 effort 不得被连带修改")
		})
	}
}

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_SystemWithBodyKept(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"non_empty_string", `"You are helpful."`},
		{"text_block", `[{"type":"text","text":"hi"}]`},
		{"text_with_empty_sibling", `[{"type":"text","text":""},{"type":"text","text":"hi"}]`},
		{"future_non_text_block", `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AA=="}}]`},
		{"unknown_block", `[{"foo":"bar"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"messages":[{"role":"system","content":` + tc.content + `,"output_config":{"effort":"high"}}]}`)
			out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
			require.True(t, changed)

			msgs := gjson.GetBytes(out, "messages").Array()
			require.Len(t, msgs, 1, "有正文 system 不得整条删除")
			require.Equal(t, "system", msgs[0].Get("role").String())
			require.False(t, msgs[0].Get("output_config").Exists(),
				"有正文 system 必须剥掉 message-level output_config")
			require.Equal(t, tc.content, msgs[0].Get("content").Raw,
				"正文必须原样保留；非 text / 未知内容块不得被误当空而删除")
		})
	}
}

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_ByteNoopWhenBetaPresent(t *testing.T) {
	body := []byte(`{"model":"claude-opus-5","output_config":{"effort":"high"},"messages":[{"role":"system","content":[],"output_config":{"effort":"high"}},{"role":"user","content":"hi"}]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body,
		"claude-code-20250219,oauth-2025-04-20,"+claude.BetaMidConversationOutputConfig)
	require.False(t, changed, "header 含 mid-conversation-output-config beta → 完全保留")
	require.True(t, bytes.Equal(body, out), "含 beta 时必须字节 no-op")
}

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_NoFieldByteNoop(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"model":"claude-opus-5","output_config":{"effort":"high"},"messages":[{"role":"system","content":[]},{"role":"user","content":"hi"}]}`),
		[]byte(`{"model":"claude-opus-5","messages":[{"role":"user","content":"hi"}]}`),
	}
	for i, body := range bodies {
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
		require.Falsef(t, changed, "case %d: 无 message-level output_config → 不得改动", i)
		require.Truef(t, bytes.Equal(body, out), "case %d: 必须字节 no-op", i)
	}
}

func TestSanitizeAnthropicBodyForBetaTokens_MidConversationOutputConfig_Idempotent(t *testing.T) {
	body := []byte(`{"model":"claude-opus-5","output_config":{"effort":"high"},"messages":[` +
		`{"role":"system","content":[],"output_config":{"effort":"high"}},` +
		`{"role":"system","content":"sys","output_config":{"effort":"high"}},` +
		`{"role":"user","content":"hi"}]}`)

	first, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
	require.True(t, changed)

	second, changedAgain := sanitizeAnthropicBodyForBetaTokens(first, "oauth-2025-04-20")
	require.False(t, changedAgain, "二次净化必须为 no-op（幂等）")
	require.True(t, bytes.Equal(first, second))
}

func TestBuildUpstreamRequestOAuthMimic_MidConversationOutputConfig(t *testing.T) {
	cases := []struct {
		name              string
		policyFilterDrops bool
		wantHeaderHasBeta bool
		wantMsgLen        int
		wantFieldOnFirst  bool
	}{
		{"default_mimic_keeps_beta", false, true, 2, true},
		{"policy_filter_drops_beta", true, false, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			if tc.policyFilterDrops {
				c.Set(betaPolicyFilterSetKey, map[string]struct{}{claude.BetaMidConversationOutputConfig: {}})
			}

			account := &Account{ID: 701, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
				Credentials: map[string]any{"access_token": "oauth-tok"},
				Status:      StatusActive,
				Schedulable: true,
			}
			body := []byte(`{"model":"claude-opus-5","output_config":{"effort":"high"},"messages":[` +
				`{"role":"system","content":[],"output_config":{"effort":"high"}},` +
				`{"role":"user","content":"hello"}]}`)

			svc := &GatewayService{cfg: &config.Config{}}
			req, _, err := svc.buildUpstreamRequest(
				context.Background(), c, account, body,
				"oauth-tok", "oauth", "claude-opus-5", false, true,
			)
			require.NoError(t, err)

			outBody := readUpstreamBodyForTest(t, req)
			outBeta := getHeaderRaw(req.Header, "anthropic-beta")
			require.Equalf(t, tc.wantHeaderHasBeta,
				anthropicBetaTokensContains(outBeta, claude.BetaMidConversationOutputConfig),
				"outgoing anthropic-beta 必须与 filter 结果一致（outgoing beta=%q）", outBeta)

			msgs := gjson.GetBytes(outBody, "messages").Array()
			require.Len(t, msgs, tc.wantMsgLen,
				"空控制 system 消息：header 带 beta 保留 / 缺 beta 整条删除")
			require.Equal(t, tc.wantFieldOnFirst, msgs[0].Get("output_config").Exists())

			var userContent string
			for _, m := range msgs {
				if m.Get("role").String() == "user" {
					userContent = m.Get("content").String()
				}
			}
			require.Equal(t, "hello", userContent, "用户消息文本必须始终保留")

			require.Equal(t, "high", gjson.GetBytes(outBody, "output_config.effort").String(),
				"顶层 effort 不受该 beta 约束")
		})
	}
}

func TestBuildUpstreamRequestAnthropicAPIKeyPassthrough_MidConversationOutputConfigConsistentWithClientHeader(t *testing.T) {
	cases := []struct {
		name       string
		clientBeta string
		wantField  bool
		wantMsgLen int
	}{
		{
			name:       "client_header_has_beta",
			clientBeta: "oauth-2025-04-20," + claude.BetaMidConversationOutputConfig,
			wantField:  true,
			wantMsgLen: 2,
		},
		{
			name:       "client_header_missing_beta",
			clientBeta: "oauth-2025-04-20",
			wantField:  false,
			wantMsgLen: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("Anthropic-Beta", tc.clientBeta)

			body := []byte(`{"model":"claude-opus-5","output_config":{"effort":"high"},"messages":[` +
				`{"role":"system","content":[],"output_config":{"effort":"high"}},` +
				`{"role":"user","content":"hello"}]}`)

			svc := &GatewayService{cfg: &config.Config{}}
			req, _, err := svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(
				context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token",
			)
			require.NoError(t, err)

			outBody := readUpstreamBodyForTest(t, req)
			outBeta := getHeaderRaw(req.Header, "anthropic-beta")

			require.Equalf(t, tc.wantField, anthropicBetaTokensContains(outBeta, claude.BetaMidConversationOutputConfig),
				"出站 header 必须与客户端传入的 beta 一致（outgoing beta=%q）", outBeta)
			require.Equal(t, tc.wantField, gjson.GetBytes(outBody, "messages.0.output_config").Exists(),
				"header/body 必须一致")
			require.Len(t, gjson.GetBytes(outBody, "messages").Array(), tc.wantMsgLen,
				"缺 beta 时空控制 system 消息整条删除；带 beta 时保留")
		})
	}
}
