package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNativeGeminiResponsesRequestAndSignedToolRoundTrip(t *testing.T) {
	raw := `{"candidates":[{"content":{"parts":[{"functionCall":{"id":"provider-call","name":"lookup","args":{"city":"Paris"}},"thoughtSignature":"opaque-signature"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":30,"candidatesTokenCount":5,"thoughtsTokenCount":7}}`
	response, err := nativeGeminiResponse([]byte(raw), "alias", "resp_test")
	require.NoError(t, err)
	require.Equal(t, "completed", response.Status)
	require.Len(t, response.Output, 2)
	require.Equal(t, "provider-call", response.Output[1].CallID)
	require.Equal(t, 100, response.Usage.InputTokens)
	require.Equal(t, 12, response.Usage.OutputTokens)
	require.Equal(t, 112, response.Usage.TotalTokens)
	require.Equal(t, 30, response.Usage.InputTokensDetails.CachedTokens)
	require.Equal(t, 7, response.Usage.OutputTokensDetails.ReasoningTokens)
	input := []any{map[string]any{"role": "user", "content": "Find weather"}, response.Output[0], response.Output[1], map[string]any{"type": "function_call_output", "call_id": "provider-call", "output": "sunny"}}
	body, _ := json.Marshal(map[string]any{"model": "alias", "store": false, "input": input, "instructions": "Be concise", "tools": []any{map[string]any{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}}, "tool_choice": "auto", "max_output_tokens": 42})
	payload, req, err := responsesToNativeGemini(body)
	require.NoError(t, err)
	require.Equal(t, "alias", req.Model)
	var generated map[string]any
	require.NoError(t, json.Unmarshal(payload, &generated))
	contents, ok := generated["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 3)
	modelContent, ok := contents[1].(map[string]any)
	require.True(t, ok)
	modelParts, ok := modelContent["parts"].([]any)
	require.True(t, ok)
	part, ok := modelParts[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "opaque-signature", part["thoughtSignature"])
	functionCall, ok := part["functionCall"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "provider-call", functionCall["id"])
	resultContent, ok := contents[2].(map[string]any)
	require.True(t, ok)
	resultParts, ok := resultContent["parts"].([]any)
	require.True(t, ok)
	resultPart, ok := resultParts[0].(map[string]any)
	require.True(t, ok)
	output, ok := resultPart["functionResponse"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "provider-call", output["id"])
	require.Equal(t, "lookup", output["name"])
	require.Equal(t, map[string]any{"result": "sunny"}, output["response"])
	require.Contains(t, string(payload), "parametersJsonSchema")
	require.NotContains(t, string(payload), geminiDummyThoughtSignature)
}

func TestNativeGeminiResponsesRejectUnsupportedRequests(t *testing.T) {
	for _, body := range []string{
		`{"model":"gemini","input":"hi","previous_response_id":"resp_old"}`,
		`{"model":"gemini","input":"hi","store":true}`,
		`{"model":"gemini","input":"hi","reasoning":{"effort":"high"}}`,
		`{"model":"gemini","input":"hi","parallel_tool_calls":false}`,
		`{"model":"gemini","input":"hi","text":{"format":{"type":"json_schema"}}}`,
		`{"model":"gemini","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,eA=="}]}]}`,
		`{"model":"gemini","input":[{"type":"item_reference","id":"msg_old"}]}`,
		`{"model":"gemini","input":[{"type":"function_call_output","call_id":"missing","output":"done"}]}`,
		`{"model":"gemini","input":[{"type":"function_call","call_id":"c","name":"tool","arguments":"[]"}]}`,
		`{"model":"gemini","input":"hi","tools":[{"type":"web_search"}]}`,
		`{"model":"gemini","input":"hi","tools":[{"type":"function","name":"f","strict":true}]}`,
		`{"model":"gemini","input":"hi","tool_choice":{"type":"function","name":"missing"}}`,
		`{"model":"gemini","input":"hi","max_output_tokens":0}`,
	} {
		t.Run(body, func(t *testing.T) { _, _, err := responsesToNativeGemini([]byte(body)); require.Error(t, err) })
	}
}

func TestNativeGeminiResponsesRejectNullMessageContent(t *testing.T) {
	for _, role := range []string{"user", "assistant", "system", "developer"} {
		t.Run(role, func(t *testing.T) {
			body := []byte(`{"model":"gemini","input":[{"role":"user","content":"hello"},{"role":"` + role + `","content":null}]}`)
			payload, req, err := responsesToNativeGemini(body)
			require.ErrorContains(t, err, "invalid text content")
			require.Nil(t, payload)
			require.Nil(t, req)
		})
	}
}

func TestNativeGeminiResponsesTermination(t *testing.T) {
	for _, tc := range []struct{ finish, status, reason string }{{"STOP", "completed", ""}, {"MAX_TOKENS", "incomplete", "max_output_tokens"}, {"SAFETY", "incomplete", "content_filter"}, {"MALFORMED_FUNCTION_CALL", "failed", ""}, {"", "failed", ""}} {
		t.Run(tc.finish, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"finishReason": tc.finish, "content": map[string]any{"parts": []any{map[string]any{"text": "hi"}}}}}})
			response, err := nativeGeminiResponse(raw, "gemini", "resp_test")
			require.NoError(t, err)
			require.Equal(t, tc.status, response.Status)
			if tc.reason != "" {
				require.Equal(t, tc.reason, response.IncompleteDetails.Reason)
			}
		})
	}
	response, err := nativeGeminiResponse([]byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`), "gemini", "resp_test")
	require.NoError(t, err)
	require.Equal(t, "incomplete", response.Status)
	_, err = nativeGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"data":"x"}}]},"finishReason":"STOP"}]}`), "gemini", "resp_test")
	require.Error(t, err)
}

func nativeGeminiTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, w
}
func nativeGeminiTestEvents(t *testing.T, raw string) []apicompat.ResponsesStreamEvent {
	t.Helper()
	var result []apicompat.ResponsesStreamEvent
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "data: ") {
			var event apicompat.ResponsesStreamEvent
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
			require.Equal(t, len(result), event.SequenceNumber)
			result = append(result, event)
		}
	}
	return result
}

func TestNativeGeminiResponsesStreamLifecycle(t *testing.T) {
	c, w := nativeGeminiTestContext()
	sse := `data: {"candidates":[{"content":{"parts":[{"text":"Hello "}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"world"},{"functionCall":{"id":"call_1","name":"lookup","args":{"q":"x"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3,"thoughtsTokenCount":2}}

`
	response, first, err := streamNativeGeminiResponses(context.Background(), c, strings.NewReader(sse), "alias", "resp_test", time.Now())
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Len(t, response.Output, 3)
	require.Equal(t, "Hello ", response.Output[0].Content[0].Text)
	require.Equal(t, "world", response.Output[1].Content[0].Text)
	require.Equal(t, "call_1", response.Output[2].CallID)
	events := nativeGeminiTestEvents(t, w.Body.String())
	require.Equal(t, "response.created", events[0].Type)
	require.Equal(t, "response.in_progress", events[1].Type)
	require.Equal(t, "response.completed", events[len(events)-1].Type)
	require.Equal(t, 5, events[len(events)-1].Response.Usage.OutputTokens)
	for _, item := range response.Output {
		var added, done bool
		for _, event := range events {
			if event.Item != nil && event.Item.ID == item.ID {
				if event.Type == "response.output_item.added" {
					added = true
				}
				if event.Type == "response.output_item.done" {
					require.True(t, added)
					done = true
				}
			}
		}
		require.True(t, done)
	}
}

func TestNativeGeminiResponsesStreamFailureAndCancellation(t *testing.T) {
	for _, sse := range []string{`data: {"candidates":[{"content":{"parts":[{"text":"partial"}]}}]}` + "\n\n", "data: {broken\n\n", "data: {\"error\":{\"code\":500}}\n\n"} {
		c, w := nativeGeminiTestContext()
		_, _, err := streamNativeGeminiResponses(context.Background(), c, strings.NewReader(sse), "gemini", "resp_test", time.Now())
		require.Error(t, err)
		events := nativeGeminiTestEvents(t, w.Body.String())
		require.Equal(t, "response.failed", events[len(events)-1].Type)
		require.NotContains(t, w.Body.String(), "response.completed")
	}
	c, _ := nativeGeminiTestContext()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := streamNativeGeminiResponses(ctx, c, strings.NewReader("data: {}\n\n"), "gemini", "resp_test", time.Now())
	require.ErrorIs(t, err, context.Canceled)
	c, w := nativeGeminiTestContext()
	_, _, err = streamNativeGeminiResponses(context.Background(), c, strings.NewReader("data: {\"candidates\":[{\"finishReason\":\"MAX_TOKENS\"}]}\n\n"), "gemini", "resp_test", time.Now())
	require.NoError(t, err)
	require.Contains(t, w.Body.String(), "response.incomplete")
}

type nativeGeminiResponsesUpstream struct {
	req      *http.Request
	body     []byte
	status   int
	response string
	err      error
}

func (u *nativeGeminiResponsesUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.req = req
	u.body, _ = io.ReadAll(req.Body)
	if u.err != nil {
		return nil, u.err
	}
	return &http.Response{StatusCode: u.status, Header: http.Header{"X-Goog-Request-Id": []string{"request-test"}}, Body: io.NopCloser(strings.NewReader(u.response))}, nil
}

func TestNativeGeminiResponsesForwardTransportAndAlias(t *testing.T) {
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: `{"candidates":[{"content":{"parts":[{"text":"OK"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":3,"candidatesTokenCount":2}}`}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"alias": "gemini-qualified"}}}
	c, w := nativeGeminiTestContext()
	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"alias","input":"hi","store":false}`))
	require.NoError(t, err)
	require.Equal(t, "/v1beta/models/gemini-qualified:generateContent", upstream.req.URL.Path)
	require.Equal(t, "test-key", upstream.req.Header.Get("x-goog-api-key"))
	require.Contains(t, string(upstream.body), "contents")
	require.NotContains(t, string(upstream.body), "messages")
	require.Equal(t, "alias", result.Model)
	require.Equal(t, "gemini-qualified", result.UpstreamModel)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.Equal(t, 200, w.Code)
	upstream.status = 400
	upstream.response = `{"error":{"code":400,"message":"Invalid function schema","status":"INVALID_ARGUMENT"}}`
	c, w = nativeGeminiTestContext()
	_, err = svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"alias","input":"hi"}`))
	require.Error(t, err)
	require.Equal(t, 400, w.Code)
	upstream.status = 503
	c, _ = nativeGeminiTestContext()
	_, err = svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"alias","input":"hi"}`))
	var failover *UpstreamFailoverError
	require.True(t, errors.As(err, &failover))
}

func TestNativeGeminiResponsesPreflightSubset(t *testing.T) {
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	features := gatewayAccountFeaturesWithMetadata(account, "gemini", "responses", map[string]UpstreamModelMetadata{"gemini": {InputModalities: []string{"text", "image"}, SupportedReasoningLevels: []string{"high"}}})
	require.Equal(t, gatewayFeatureSupported, features.Features["protocol"])
	require.Equal(t, gatewayFeatureSupported, features.Features["streaming"])
	require.Equal(t, gatewayFeatureSupported, features.Features["text_input"])
	for _, key := range []string{"image_input", "audio_input", "reasoning", "web_search", "tool_discovery"} {
		require.Equal(t, gatewayFeatureUnsupported, features.Features[key])
	}
	require.Nil(t, features.ReasoningEfforts)
	account.Type = AccountTypeOAuth
	require.Equal(t, gatewayFeatureUnsupported, gatewayProtocolFeature(account, "responses"))
}

func (u *nativeGeminiResponsesUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func TestNativeGeminiResponsesSignedTextHistory(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"text":"private thought","thought":true,"thoughtSignature":"think-sig"},{"text":"answer","thoughtSignature":"text-sig"}]},"finishReason":"STOP"}]}`)
	response, err := nativeGeminiResponse(raw, "gemini", "resp_test")
	require.NoError(t, err)
	require.Len(t, response.Output, 3)
	input := []any{map[string]any{"role": "user", "content": "hi"}, response.Output[0], response.Output[1], response.Output[2], map[string]any{"role": "user", "content": "continue"}}
	body, _ := json.Marshal(map[string]any{"model": "gemini", "input": input})
	converted, _, err := responsesToNativeGemini(body)
	require.NoError(t, err)
	require.Contains(t, string(converted), `"thoughtSignature":"think-sig"`)
	require.Contains(t, string(converted), `"thoughtSignature":"text-sig"`)
	response.Output[2].Content[0].Text = "changed"
	input[3] = response.Output[2]
	body, _ = json.Marshal(map[string]any{"model": "gemini", "input": input})
	_, _, err = responsesToNativeGemini(body)
	require.ErrorContains(t, err, "signed message changed")
}

func TestNativeGeminiResponsesRejectSignedCallIDMismatch(t *testing.T) {
	response, err := nativeGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"id":"original","name":"f","args":{}},"thoughtSignature":"signature"}]},"finishReason":"STOP"}]}`), "gemini", "resp_test")
	require.NoError(t, err)
	envelope := response.Output[0]
	data, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(envelope.EncryptedContent, geminiResponsesEnvelope))
	var altered map[string]any
	require.NoError(t, json.Unmarshal(data, &altered))
	altered["call_id"] = "changed"
	data, _ = json.Marshal(altered)
	envelope.EncryptedContent = geminiResponsesEnvelope + base64.RawURLEncoding.EncodeToString(data)
	call := response.Output[1]
	call.CallID = "changed"
	body, _ := json.Marshal(map[string]any{"model": "gemini", "input": []any{envelope, call}})
	_, _, err = responsesToNativeGemini(body)
	require.ErrorContains(t, err, "signed call ID changed")
}

type nativeGeminiFailWriter struct {
	gin.ResponseWriter
	writes int
}

func (w *nativeGeminiFailWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes >= 3 {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseWriter.Write(data)
}
func TestNativeGeminiResponsesClientDisconnectKeepsUsage(t *testing.T) {
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}],\"modelVersion\":\"gemini-actual\",\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5}}\n\n"}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	c, w := nativeGeminiTestContext()
	c.Writer = &nativeGeminiFailWriter{ResponseWriter: c.Writer}
	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","stream":true}`))
	require.NoError(t, err)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, "gemini-actual", result.UpstreamResponseModel)
	require.NotContains(t, w.Body.String(), "response.completed")
}

func TestNativeGeminiResponsesPreservesLargeArgumentsAndRejectsDuplicateCalls(t *testing.T) {
	response, err := nativeGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"id":"c","name":"f","args":{"id":9007199254740993}}}]},"finishReason":"STOP"}]}`), "gemini", "resp_test")
	require.NoError(t, err)
	require.Contains(t, response.Output[0].Arguments, "9007199254740993")
	body, _ := json.Marshal(map[string]any{"model": "gemini", "input": response.Output})
	converted, _, err := responsesToNativeGemini(body)
	require.NoError(t, err)
	require.Contains(t, string(converted), "9007199254740993")
	duplicate := `{"candidates":[{"content":{"parts":[{"functionCall":{"id":"c","name":"f","args":{}}},{"functionCall":{"id":"c","name":"f","args":{}}}]},"finishReason":"STOP"}]}`
	_, err = nativeGeminiResponse([]byte(duplicate), "gemini", "resp_test")
	require.ErrorContains(t, err, "duplicate")
	c, w := nativeGeminiTestContext()
	sse := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"id\":\"c\",\"name\":\"f\",\"args\":{}}}]}}]}\n\n"
	_, _, err = streamNativeGeminiResponses(context.Background(), c, strings.NewReader(sse+sse), "gemini", "resp_test", time.Now())
	require.ErrorContains(t, err, "duplicate")
	require.Equal(t, 1, strings.Count(w.Body.String(), `"type":"response.function_call_arguments.done"`))
	require.Contains(t, w.Body.String(), "response.failed")
}

func TestNativeGeminiResponsesBadInputAfterHeartbeat(t *testing.T) {
	c, w := nativeGeminiTestContext()
	c.Header("Content-Type", "text/event-stream")
	_, _ = c.Writer.WriteString(": ping\n\n")
	c.Writer.Flush()
	svc := &GeminiMessagesCompatService{}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	_, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","reasoning":{"effort":"high"}}`))
	require.Error(t, err)
	require.Equal(t, 200, w.Code)
	require.Equal(t, 1, strings.Count(w.Body.String(), "event: response.failed"))
	require.True(t, IsResponseCommitted(c))
	require.NotContains(t, w.Body.String(), "response.completed")
}

func TestNativeGeminiResponsesMalformedPartsFailClosed(t *testing.T) {
	for _, parts := range []string{`[42]`, `{}`, `[{"text":"x","functionCall":{"name":"f"}}]`, `[{"text":"x","inlineData":{}}]`} {
		raw := `{"candidates":[{"finishReason":"STOP","content":{"parts":` + parts + `}}]}`
		_, err := nativeGeminiResponse([]byte(raw), "gemini", "resp_test")
		require.Error(t, err)
		c, w := nativeGeminiTestContext()
		_, _, err = streamNativeGeminiResponses(context.Background(), c, strings.NewReader("data: "+raw+"\n\n"), "gemini", "resp_test", time.Now())
		require.Error(t, err)
		require.NotContains(t, w.Body.String(), "response.completed")
		require.Contains(t, w.Body.String(), "response.failed")
	}
	response, err := nativeGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"text":"answer","thoughtSignature":"sig"}]},"finishReason":"STOP"}]}`), "gemini", "resp_test")
	require.NoError(t, err)
	body, _ := json.Marshal(map[string]any{"model": "gemini", "input": []any{response.Output[0], map[string]any{"role": "user", "content": "hi"}}})
	_, _, err = responsesToNativeGemini(body)
	require.ErrorContains(t, err, "unpaired")
	body, _ = json.Marshal(map[string]any{"model": "gemini", "input": []any{response.Output[0], response.Output[0], response.Output[1]}})
	_, _, err = responsesToNativeGemini(body)
	require.ErrorContains(t, err, "duplicate")
}

func TestNativeGeminiResponsesFailedStreamPreservesAccounting(t *testing.T) {
	for _, finish := range []string{"MALFORMED_FUNCTION_CALL", ""} {
		raw := `{"candidates":[{"finishReason":"` + finish + `","content":{"parts":[{"text":"partial"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}`
		upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: " + raw + "\n\n"}
		svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
		account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
		c, w := nativeGeminiTestContext()
		result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","stream":true}`))
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 10, result.Usage.InputTokens)
		require.Equal(t, 20, result.Usage.OutputTokens)
		require.Contains(t, w.Body.String(), "response.failed")
		require.NotContains(t, w.Body.String(), "response.completed")
		require.True(t, IsResponseCommitted(c))
	}
}

func TestNativeGeminiResponsesMalformedFramePreservesAccounting(t *testing.T) {
	first := `{"candidates":[{"content":{"parts":[{"text":"partial"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`
	last := `{"candidates":[{"finishReason":"STOP","content":{"parts":[42]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}`
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: " + first + "\n\ndata: " + last + "\n\n"}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	c, w := nativeGeminiTestContext()
	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","stream":true}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "failed", result.ResponseStatus)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 20, result.Usage.OutputTokens)
	require.True(t, IsResponseCommitted(c))
	events := nativeGeminiTestEvents(t, w.Body.String())
	terminal := events[len(events)-1]
	require.Equal(t, "response.failed", terminal.Type)
	require.Equal(t, "failed", terminal.Response.Status)
	require.Equal(t, 20, terminal.Response.Usage.OutputTokens)
	require.Len(t, terminal.Response.Output, 1)
	require.Equal(t, "partial", terminal.Response.Output[0].Content[0].Text)
	require.Equal(t, 1, strings.Count(w.Body.String(), `"type":"response.output_text.delta"`))
	require.NotContains(t, w.Body.String(), "response.completed")
}

func TestNativeGeminiResponsesErrorFrameUsage(t *testing.T) {
	for _, tc := range []struct {
		name, metadata        string
		wantInput, wantOutput int
	}{
		{"empty", `{}`, 10, 2},
		{"invalid strings", `{"promptTokenCount":"bad","candidatesTokenCount":"bad"}`, 10, 2},
		{"fractional", `{"promptTokenCount":10,"candidatesTokenCount":1.5}`, 10, 2},
		{"negative", `{"promptTokenCount":10,"candidatesTokenCount":-1}`, 10, 2},
		{"null", `null`, 10, 2},
		{"unrecognized counts", `{"totalTokenCount":12}`, 10, 2},
		{"valid cumulative counts", `{"promptTokenCount":10,"candidatesTokenCount":20}`, 10, 20},
		{"valid zero", `{"promptTokenCount":0,"candidatesTokenCount":0}`, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := `{"candidates":[{"content":{"parts":[{"text":"partial"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`
			last := `{"error":{"code":500},"usageMetadata":` + tc.metadata + `}`
			upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: " + first + "\n\ndata: " + last + "\n\n"}
			svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
			c, w := nativeGeminiTestContext()
			result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","stream":true}`))
			require.NoError(t, err)
			require.Equal(t, "failed", result.ResponseStatus)
			require.Equal(t, tc.wantInput, result.Usage.InputTokens)
			require.Equal(t, tc.wantOutput, result.Usage.OutputTokens)
			require.Contains(t, w.Body.String(), "response.failed")
			require.NotContains(t, w.Body.String(), "response.completed")
		})
	}
}

func TestNativeGeminiResponsesRejectsNestedCapabilities(t *testing.T) {
	for _, body := range []string{
		`{"model":"gemini","input":"hi","tools":[{"type":"function","name":"f","defer_loading":true}]}`,
		`{"model":"gemini","input":[{"role":"user","content":"hi","audio":{}}]}`,
		`{"model":"gemini","input":[{"role":"user","content":[{"type":"input_text","text":"hi","image_url":"secret"}]}]}`,
	} {
		_, _, err := responsesToNativeGemini([]byte(body))
		require.Error(t, err)
	}
}

func TestNativeGeminiResponsesTerminalOnlyChunk(t *testing.T) {
	text := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"complete text\"}]}}]}\n\n"
	terminal := "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2}}\n\n"
	c, w := nativeGeminiTestContext()
	response, _, err := streamNativeGeminiResponses(context.Background(), c, strings.NewReader(text+terminal), "gemini", "resp_test", time.Now())
	require.NoError(t, err)
	require.Equal(t, "completed", response.Status)
	require.Equal(t, "complete text", response.Output[0].Content[0].Text)
	require.Equal(t, 2, response.Usage.OutputTokens)
	require.Contains(t, w.Body.String(), "response.completed")
	c, w = nativeGeminiTestContext()
	_, _, err = streamNativeGeminiResponses(context.Background(), c, strings.NewReader(terminal), "gemini", "resp_test", time.Now())
	require.Error(t, err)
	require.Contains(t, w.Body.String(), "response.failed")
	require.NotContains(t, w.Body.String(), "response.completed")
}

func TestNativeGeminiResponsesDuplicateConsumedEnvelope(t *testing.T) {
	response, err := nativeGeminiResponse([]byte(`{"candidates":[{"content":{"parts":[{"text":"answer","thoughtSignature":"sig"}]},"finishReason":"STOP"}]}`), "gemini", "resp_test")
	require.NoError(t, err)
	body, _ := json.Marshal(map[string]any{"model": "gemini", "input": []any{response.Output[0], response.Output[1], response.Output[0], response.Output[1]}})
	_, _, err = responsesToNativeGemini(body)
	require.ErrorContains(t, err, "duplicate")
}

func TestNativeGeminiResponsesNonstreamFailureOpsAndAccounting(t *testing.T) {
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: `{"candidates":[{"finishReason":"MALFORMED_FUNCTION_CALL"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}`}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	c, w := nativeGeminiTestContext()
	result, err := svc.ForwardAsResponses(context.Background(), c, &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}, []byte(`{"model":"gemini","input":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 20, result.Usage.OutputTokens)
	require.Equal(t, "failed", result.ResponseStatus)
	message, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "Gemini generation failed", message)
	require.Contains(t, w.Body.String(), `"status":"failed"`)
	require.NotContains(t, w.Body.String(), `"status":"completed"`)
}

type nativeGeminiOperatorCache struct {
	gatewayCapabilitySchedulerCacheStub
	account *Account
}

func (c *nativeGeminiOperatorCache) GetAccount(context.Context, int64) (*Account, error) {
	return c.account, nil
}
func TestNativeGeminiResponsesOperatorAssistantRelay(t *testing.T) {
	group := Group{ID: 42, Platform: PlatformGemini, Status: StatusActive, Hydrated: true}
	account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "test"}, AccountGroups: []AccountGroup{{GroupID: 42, AccountID: 1}}}
	snapshot := gatewayCapabilitySchedulerSnapshot(42, PlatformGemini, account)
	cache, ok := snapshot.cache.(*gatewayCapabilitySchedulerCacheStub)
	require.True(t, ok)
	snapshot.cache = &nativeGeminiOperatorCache{gatewayCapabilitySchedulerCacheStub: *cache, account: account}
	gateway := &GatewayService{schedulerSnapshot: snapshot}
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"operator OK\"}]},\"finishReason\":\"STOP\"}]}\n\n"}
	gemini := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	assistant := NewOperatorAssistantService(nil, nil, nil, gateway, nil, nil, gemini, BuildInfo{}, nil)
	c, w := nativeGeminiTestContext()
	ctx := context.WithValue(context.Background(), ctxkey.Group, &group)
	err := assistant.relayExistingGateway(ctx, c, operatorAssistantCandidate{Option: OperatorAssistantModelOption{GroupID: 42, Model: "gemini", Platform: PlatformGemini}, Group: group}, []byte(`{"model":"gemini","input":"hi","stream":true,"store":false}`), 9)
	require.NoError(t, err)
	require.NotNil(t, upstream.req)
	require.Equal(t, "/v1beta/models/gemini:streamGenerateContent", upstream.req.URL.Path)
	require.Contains(t, w.Body.String(), "operator OK")
	require.Contains(t, w.Body.String(), "response.completed")

	upstream.response = "data: {\"candidates\":[{\"finishReason\":\"MALFORMED_FUNCTION_CALL\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2}}\n\n"
	c, w = nativeGeminiTestContext()
	err = assistant.relayExistingGateway(ctx, c, operatorAssistantCandidate{Option: OperatorAssistantModelOption{GroupID: 42, Model: "gemini", Platform: PlatformGemini}, Group: group}, []byte(`{"model":"gemini","input":"hi","stream":true,"store":false}`), 9)
	require.ErrorContains(t, err, "native Gemini generation failed")
	require.Contains(t, w.Body.String(), "response.failed")
	require.NotContains(t, w.Body.String(), "response.completed")

	account.AccountGroups = append(account.AccountGroups, AccountGroup{GroupID: 43, AccountID: 1})
	reader := &operatorAssistantReaderStub{
		groups:   []Group{group, {ID: 43, Platform: PlatformGemini, Status: StatusActive}},
		models:   map[int64][]string{42: {"gemini"}, 43: {"gemini"}},
		accounts: []Account{*account},
	}
	orchestrator := operatorAssistantTestService(reader)
	relay := assistant.relay
	attempts := 0
	orchestrator.relay = func(ctx context.Context, c *gin.Context, candidate operatorAssistantCandidate, body []byte, adminID int64) error {
		attempts++
		return relay(ctx, c, candidate, body, adminID)
	}
	c, w = nativeGeminiTestContext()
	c.Request = c.Request.WithContext(ctx)
	run, err := orchestrator.Stream(c, 9, OperatorAssistantRequest{Model: "auto", Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}}})
	require.Error(t, err)
	require.True(t, run.Started)
	require.Equal(t, 1, attempts)
	require.Contains(t, w.Body.String(), "response.failed")
	require.NotContains(t, w.Body.String(), "response.completed")
}
