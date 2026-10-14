package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

const geminiNamespaceTools = `[{"type":"namespace","name":"files","description":"Owned files","tools":[{"type":"function","name":"read","parameters":{"type":"object","properties":{"line":{"type":"integer"}}}}]}]`

func TestNativeGeminiResponsesSupportedOutputControls(t *testing.T) {
	_, _, err := responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi","include":["reasoning.encrypted_content"],"text":{"format":{"type":"text"}},"parallel_tool_calls":true}`))
	require.NoError(t, err)
	for _, control := range []string{
		`"include":["web_search_call.action.sources"]`,
		`"include":"reasoning.encrypted_content"`,
		`"text":{"format":{"type":"text"},"verbosity":"low"}`,
		`"text":{"format":{"type":"json_object"}}`,
		`"text":null`,
		`"parallel_tool_calls":false`,
		`"parallel_tool_calls":null`,
	} {
		_, _, err := responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi",` + control + `}`))
		require.Error(t, err)
	}
}

func TestNativeGeminiResponsesNamespaceSignedRoundTrip(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			raw := `{"candidates":[{"content":{"parts":[{"functionCall":{"id":"provider-call","name":"files__read","args":{"line":9007199254740993}},"thoughtSignature":"opaque-signature"}]},"finishReason":"STOP"}],"modelVersion":"gemini-actual","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`
			response := raw
			if stream {
				response = "data: " + raw + "\n\n"
			}
			upstream := &nativeGeminiResponsesUpstream{status: 200, response: response}
			svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic"}}
			body := []byte(fmt.Sprintf(`{"model":"alias","input":"Read owned line","stream":%t,"tools":%s,"tool_choice":{"type":"function","namespace":"files","name":"read"}}`, stream, geminiNamespaceTools))
			c, w := nativeGeminiTestContext()
			result, err := svc.ForwardAsResponses(context.Background(), c, account, body)
			require.NoError(t, err)
			require.Equal(t, "gemini-actual", result.UpstreamResponseModel)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Contains(t, string(upstream.body), `"allowedFunctionNames":["files__read"]`)
			require.Contains(t, string(upstream.body), `files: Owned files`)
			var completed apicompat.ResponsesResponse
			if stream {
				sequence := 0
				for _, line := range strings.Split(w.Body.String(), "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var event apicompat.ResponsesStreamEvent
					require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
					require.Equal(t, sequence, event.SequenceNumber)
					sequence++
					if event.Item != nil && event.Item.Type == "function_call" {
						require.Equal(t, "files", event.Item.Namespace)
						require.Equal(t, "read", event.Item.Name)
						require.Equal(t, "provider-call", event.Item.CallID)
					}
					if event.Type == "response.completed" {
						completed = *event.Response
					}
				}
			} else {
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &completed))
			}
			require.Equal(t, "completed", completed.Status)
			require.Len(t, completed.Output, 2)
			require.Equal(t, "files", completed.Output[1].Namespace)
			require.Equal(t, "read", completed.Output[1].Name)
			require.Contains(t, completed.Output[1].Arguments, "9007199254740993")
			var tools any
			require.NoError(t, json.Unmarshal([]byte(geminiNamespaceTools), &tools))
			input := []any{map[string]any{"role": "user", "content": "Read owned line"}, completed.Output[0], completed.Output[1], map[string]any{"type": "function_call_output", "call_id": "provider-call", "output": "found"}}
			replay, err := json.Marshal(map[string]any{"model": "alias", "tools": tools, "input": input})
			require.NoError(t, err)
			payload, _, err := responsesToNativeGemini(replay)
			require.NoError(t, err)
			require.Contains(t, string(payload), `"thoughtSignature":"opaque-signature"`)
			require.Contains(t, string(payload), `"name":"files__read"`)
			require.Contains(t, string(payload), `"id":"provider-call"`)
			require.Contains(t, string(payload), "9007199254740993")
		})
	}
}

func TestNativeGeminiResponsesRejectLossyNamespaceLowering(t *testing.T) {
	for _, tools := range []string{
		`[{"type":"namespace","name":"n","tools":[]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"custom","name":"exec"},{"type":"function","name":"f"}]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f"},{"type":"function","name":"f"}]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f","defer_loading":true}]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f","strict":true}]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f"}]},{"type":"function","name":"n__f"}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f"}]},{"type":"namespace","name":"n","tools":[{"type":"function","name":"g"}]}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f"}],"defer_loading":true}]`,
		`[{"type":"namespace","name":"n","tools":[{"type":"function","name":"f"}]},{"type":" namespace ","name":"other","tools":[{"type":"custom","name":"exec"},{"type":"function","name":"f"}]}]`,
	} {
		t.Run(tools, func(t *testing.T) {
			_, _, err := responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi","tools":` + tools + `}`))
			require.Error(t, err)
		})
	}
	_, _, err := responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi","tools":` + geminiNamespaceTools + `,"tool_choice":{"type":"namespace","name":"files"}}`))
	require.ErrorContains(t, err, "namespace-only tool_choice")
	_, _, err = responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi","tools":` + geminiNamespaceTools + `,"tool_choice":{"type":" namespace ","name":"files"}}`))
	require.ErrorContains(t, err, "namespace-only tool_choice")
	for _, choice := range []string{
		`{"type":"function","namespace":"missing","name":"plain"}`,
		`{"type":"function","namespace":"files","name":"plain"}`,
		`{"type":"function","name":"plain","unexpected":true}`,
	} {
		_, _, err = responsesToNativeGemini([]byte(`{"model":"gemini","input":"hi","tools":[{"type":"function","name":"plain"},` + strings.Trim(geminiNamespaceTools, "[]") + `],"tool_choice":` + choice + `}`))
		require.Error(t, err)
	}
}

func TestNativeGeminiResponsesStreamRestoresNamespaceOnlyOnce(t *testing.T) {
	tools := `[{"type":"namespace","name":"a","tools":[{"type":"function","name":"b__c"}]},{"type":"namespace","name":"b","tools":[{"type":"function","name":"c"}]}]`
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: "data: " + `{"candidates":[{"content":{"parts":[{"functionCall":{"id":"call","name":"a__b__c","args":{}},"thoughtSignature":"sig"}]},"finishReason":"STOP"}]}` + "\n\n"}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic"}}
	c, w := nativeGeminiTestContext()
	_, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"gemini","input":"hi","stream":true,"tools":`+tools+`}`))
	require.NoError(t, err)
	for _, line := range strings.Split(w.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event apicompat.ResponsesStreamEvent
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
		if event.Item != nil && event.Item.Type == "function_call" {
			require.Equal(t, "a", event.Item.Namespace)
			require.Equal(t, "b__c", event.Item.Name)
		}
		if event.Type == "response.completed" {
			call := event.Response.Output[1]
			require.Equal(t, "a", call.Namespace)
			require.Equal(t, "b__c", call.Name)
			var declarations any
			require.NoError(t, json.Unmarshal([]byte(tools), &declarations))
			replay, err := json.Marshal(map[string]any{"model": "gemini", "tools": declarations, "input": event.Response.Output})
			require.NoError(t, err)
			_, _, err = responsesToNativeGemini(replay)
			require.NoError(t, err)
		}
	}
}

func TestNativeGeminiResponsesThoughtOnlyStopFailsWithUsage(t *testing.T) {
	raw := `{"candidates":[{"content":{"parts":[{"text":"private thought","thought":true,"thoughtSignature":"sig"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"thoughtsTokenCount":3}}`
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			response := raw
			if stream {
				response = "data: " + raw + "\n\n"
			}
			upstream := &nativeGeminiResponsesUpstream{status: 200, response: response}
			svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic"}}
			c, w := nativeGeminiTestContext()
			result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(fmt.Sprintf(`{"model":"gemini","input":"hi","stream":%t}`, stream)))
			require.NoError(t, err)
			require.Equal(t, "failed", result.ResponseStatus)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Equal(t, 3, result.Usage.OutputTokens)
			require.NotContains(t, w.Body.String(), `"status":"completed"`)
		})
	}
}

func TestNativeGeminiResponsesMalformedBufferedOutputKeepsUsage(t *testing.T) {
	upstream := &nativeGeminiResponsesUpstream{status: 200, response: `{"candidates":[{"content":{"parts":[{"text":"answer","thoughtSignature":123}]},"finishReason":"STOP"}],"modelVersion":"gemini-actual","usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":2,"candidatesTokenCount":3}}`}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic"}}
	c, w := nativeGeminiTestContext()
	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"alias","input":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, 502, w.Code)
	require.Equal(t, "failed", result.ResponseStatus)
	require.Equal(t, "gemini-actual", result.UpstreamResponseModel)
	require.Equal(t, 8, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.NotContains(t, w.Body.String(), "thoughtSignature")
	require.Contains(t, w.Body.String(), `"status":"failed"`)
}
