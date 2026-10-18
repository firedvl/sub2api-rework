//go:build unit

package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

const claudeProbeStart = "data: {\"type\":\"message_start\",\"message\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"stop_reason\":null}}\n\n"
const claudeProbeText = "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"
const claudeProbeTerminal = "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":1}}\n\n"
const claudeProbeStop = "data: {\"type\":\"message_stop\"}"

func TestAccountTestService_ClaudeStreamRequiresNativeCompletion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stream  string
		success bool
	}{
		{name: "empty"},
		{name: "non SSE body", stream: "<html>upstream error</html>"},
		{name: "done only", stream: "data: [DONE]\n\n"},
		{name: "truncated after start", stream: claudeProbeStart},
		{name: "truncated after text", stream: claudeProbeStart + claudeProbeText},
		{name: "done after text", stream: claudeProbeStart + claudeProbeText + "data: [DONE]\n\n"},
		{name: "truncated after terminal delta", stream: claudeProbeStart + claudeProbeTerminal},
		{name: "stop only", stream: claudeProbeStop + "\n\n"},
		{name: "missing terminal delta", stream: claudeProbeStart + claudeProbeText + claudeProbeStop + "\n\n"},
		{name: "missing start", stream: claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "malformed start", stream: "data: {\"type\":\"message_start\"}\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "malformed content delta", stream: claudeProbeStart + "data: {\"type\":\"content_block_delta\",\"delta\":null}\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "malformed message delta", stream: claudeProbeStart + "data: {\"type\":\"message_delta\",\"delta\":null}\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "duplicate start", stream: claudeProbeStart + claudeProbeStart + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "text after terminal delta", stream: claudeProbeStart + claudeProbeTerminal + claudeProbeText + claudeProbeStop + "\n\n"},
		{name: "null stop reason", stream: claudeProbeStart + "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":null}}\n\n" + claudeProbeStop + "\n\n"},
		{name: "invalid stop reason", stream: claudeProbeStart + "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":123}}\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "malformed data", stream: claudeProbeStart + "data: {oops}\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "null data", stream: claudeProbeStart + "data: null\n\n" + claudeProbeTerminal + claudeProbeStop + "\n\n"},
		{name: "error event", stream: claudeProbeStart + "data: {\"type\":\"error\",\"error\":{\"message\":\"overloaded\"}}\n\n"},
		{name: "complete", stream: claudeProbeStart + claudeProbeText + claudeProbeTerminal + claudeProbeStop + "\n\n", success: true},
		{name: "complete without final newline", stream: claudeProbeStart + claudeProbeTerminal + claudeProbeStop, success: true},
		{name: "max tokens is terminal", stream: claudeProbeStart + strings.ReplaceAll(claudeProbeTerminal, "end_turn", "max_tokens") + claudeProbeStop + "\n\n", success: true},
		{name: "ping and comments", stream: ": keep-alive\n\ndata: {\"type\":\"ping\"}\n\n" + claudeProbeStart + claudeProbeTerminal + claudeProbeStop + "\n\n", success: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newTestContext()
			err := (&AccountTestService{}).processClaudeStream(c, strings.NewReader(tc.stream))
			if tc.success {
				require.NoError(t, err)
				require.Contains(t, recorder.Body.String(), `"type":"test_complete","success":true`)
			} else {
				require.Error(t, err)
				require.NotContains(t, recorder.Body.String(), `"success":true`)
				require.Contains(t, recorder.Body.String(), `"type":"error"`)
			}
		})
	}
}

func TestAccountTestService_ClaudeStreamReadErrorDoesNotSucceed(t *testing.T) {
	c, recorder := newTestContext()
	body := io.MultiReader(strings.NewReader(claudeProbeStart+claudeProbeText), iotest.ErrReader(errors.New("connection reset")))
	err := (&AccountTestService{}).processClaudeStream(c, body)
	require.ErrorContains(t, err, "connection reset")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_ClaudeAPIKeyProbeUsesMinimalNativeRequest(t *testing.T) {
	account := &Account{
		ID: 771, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-api-key", "base_url": "http://anthropic.example"},
	}
	upstream := &httpUpstreamRecorder{resp: newJSONResponse(http.StatusOK, claudeProbeStart+claudeProbeTerminal+claudeProbeStop)}
	svc := &AccountTestService{httpUpstream: upstream, cfg: rawChatCompletionsTestConfig()}
	c, _ := newTestContext()

	require.NoError(t, svc.testClaudeAccountConnection(c, account, "claude-sonnet-4-6"))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.lastBody, &payload))
	require.Equal(t, map[string]any{
		"model": "claude-sonnet-4-6", "max_tokens": float64(16), "stream": true,
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}, payload)
	require.Equal(t, "2023-06-01", upstream.lastReq.Header.Get("anthropic-version"))
	require.Equal(t, "http://anthropic.example/v1/messages", upstream.lastReq.URL.String())
	require.Empty(t, upstream.lastReq.Header.Get("anthropic-beta"))
	require.Equal(t, "test-api-key", upstream.lastReq.Header.Get("x-api-key"))
	for header := range claude.DefaultHeaders() {
		require.Empty(t, upstream.lastReq.Header.Get(header), header)
	}
}

func TestAccountTestService_ClaudeOAuthProbePreservesLegacyRequest(t *testing.T) {
	account := &Account{
		ID: 772, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-oauth-token"},
	}
	upstream := &httpUpstreamRecorder{resp: newJSONResponse(http.StatusOK, claudeProbeStart+claudeProbeTerminal+claudeProbeStop)}
	svc := &AccountTestService{httpUpstream: upstream}
	c, _ := newTestContext()

	require.NoError(t, svc.testClaudeAccountConnection(c, account, "claude-sonnet-4-6"))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.lastBody, &payload))
	require.Equal(t, float64(1024), payload["max_tokens"])
	require.NotEmpty(t, payload["system"])
	require.NotEmpty(t, payload["metadata"])
	require.Equal(t, "Bearer test-oauth-token", upstream.lastReq.Header.Get("Authorization"))
	for header, value := range claude.DefaultHeaders() {
		require.Equal(t, value, upstream.lastReq.Header.Get(header), header)
	}
}
