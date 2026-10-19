package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicUsageObservationStreamingEvidence(t *testing.T) {
	start := `data: {"type":"message_start","message":{"usage":{"input_tokens":8}}}` + "\n\n"
	delta := `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}` + "\n\n"
	stop := "data: {\"type\":\"message_stop\"}\n\n"
	for _, tc := range []struct {
		name, body string
		want       *bool
		wantError  bool
	}{
		{"complete", start + delta + stop, boolPtr(true), false},
		{"late_usage_after_stop", start + delta + stop + delta, boolPtr(false), false},
		{"second_message_start", start + delta + start + stop, boolPtr(false), false},
		{"invalid_start_then_valid_start", strings.Replace(start, `"input_tokens":8`, `"input_tokens":"8"`, 1) + start + delta + stop, boolPtr(false), false},
		{"invalid_final_then_valid_final", start + strings.Replace(delta, `"output_tokens":3`, `"ignored":3`, 1) + delta + stop, boolPtr(false), false},
		{"error_after_stop", start + delta + stop + "data: {\"type\":\"error\"}\n\n", boolPtr(false), false},
		{"malformed_after_stop", start + delta + stop + "data: {broken\n\n", boolPtr(false), false},
		{"observed_zero", strings.ReplaceAll(start+delta+stop, "8", "0"), boolPtr(true), false},
		{"missing_stop", start + delta, boolPtr(false), true},
		{"stop_without_final_usage", start + stop, boolPtr(false), false},
		{"malformed_terminal_json", start + delta + "event: message_stop\ndata: {broken\n\n", boolPtr(false), false},
		{"done_is_not_native_completion_evidence", start + delta + "data: [DONE]\n\n", boolPtr(false), false},
		{"string_usage", start + strings.Replace(delta, `"output_tokens":3`, `"output_tokens":"3"`, 1) + stop, boolPtr(false), false},
		{"invalid_optional_cache_count", strings.Replace(start, `"input_tokens":8`, `"input_tokens":8,"cache_read_input_tokens":"10"`, 1) + delta + stop, boolPtr(false), false},
		{"null_stop_reason", start + strings.Replace(delta, `"end_turn"`, "null", 1) + stop, boolPtr(false), false},
		{"no_usage_evidence", stop, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: 200,
				Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(tc.body))}}
			svc := newForwardPartialUsageServiceForTest(upstream)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			result, err := svc.forwardAnthropicAPIKeyPassthrough(context.Background(), c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"claude-sonnet-4-6","stream":true}`), "claude-sonnet-4-6", "claude-sonnet-4-6", true, time.Now())
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, result)
			require.Equal(t, tc.want, result.UsageObservationComplete)
			if tc.name == "complete" {
				require.Equal(t, 8, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
			}
		})
	}
}

func TestAnthropicUsageObservationNonStreamingEvidence(t *testing.T) {
	for _, tc := range []struct {
		body string
		want *bool
	}{
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0}}`, boolPtr(true)},
		{`{"type":"message","usage":{"input_tokens":8,"output_tokens":3}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":"8","output_tokens":3}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":-1}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":0.5}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":1e100}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":3,"cache_read_input_tokens":"10"}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":3,"cache_creation":{"ephemeral_5m_input_tokens":-1}}}`, boolPtr(false)},
		{`{"type":"message","stop_reason":"end_turn"}`, nil},
	} {
		t.Run(tc.body, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}
			_, complete, err := (&GatewayService{cfg: &config.Config{}}).handleNonStreamingResponseAnthropicAPIKeyPassthrough(t.Context(), resp, c, newAnthropicAPIKeyAccountForTest())
			require.NoError(t, err)
			require.Equal(t, tc.want, complete)
		})
	}
}

func TestPartialUsageObservationExcludesFailoverAndEmptyUsage(t *testing.T) {
	response := &http.Response{Header: make(http.Header)}
	observed := &streamingResult{usage: &ClaudeUsage{InputTokens: 8}}
	require.Nil(t, partialStreamUsageResult(nil, response, observed, "x", "x", time.Now(), &UpstreamFailoverError{StatusCode: 429}))
	require.Nil(t, partialStreamUsageResult(nil, response, &streamingResult{usage: &ClaudeUsage{}}, "x", "x", time.Now(), io.ErrUnexpectedEOF))
	partial := partialStreamUsageResult(nil, response, observed, "x", "x", time.Now(), io.ErrUnexpectedEOF)
	require.NotNil(t, partial)
	require.Equal(t, boolPtr(false), partial.UsageObservationComplete)
	require.Equal(t, 8, partial.Usage.InputTokens)
}
