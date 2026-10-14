package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeGeminiResponsesValidateOutputAndSignedReplay(t *testing.T) {
	for _, tc := range []struct{ name, part string }{
		{"signature number", `{"text":"answer","thoughtSignature":42}`},
		{"signature empty", `{"text":"answer","thoughtSignature":""}`},
		{"signature null", `{"text":"answer","thoughtSignature":null}`},
		{"thought string", `{"text":"answer","thought": "true","thoughtSignature":"sig"}`},
		{"thought null", `{"text":"answer","thought":null,"thoughtSignature":"sig"}`},
		{"text number", `{"text":42,"thoughtSignature":"sig"}`},
		{"text null", `{"text":null,"thoughtSignature":"sig"}`},
		{"call null", `{"functionCall":null,"thoughtSignature":"sig"}`},
		{"union neither", `{"thoughtSignature":"sig"}`},
		{"union both", `{"text":"answer","functionCall":{"name":"f"},"thoughtSignature":"sig"}`},
		{"union null call", `{"text":"answer","functionCall":null,"thoughtSignature":"sig"}`},
		{"union null text", `{"text":null,"functionCall":{"name":"f"},"thoughtSignature":"sig"}`},
		{"union wrong call", `{"text":"answer","functionCall":42,"thoughtSignature":"sig"}`},
		{"call unknown field", `{"functionCall":{"name":"f","extra":true},"thoughtSignature":"sig"}`},
		{"call id number", `{"functionCall":{"name":"f","id":42},"thoughtSignature":"sig"}`},
		{"call id empty", `{"functionCall":{"name":"f","id":""},"thoughtSignature":"sig"}`},
		{"call id null", `{"functionCall":{"name":"f","id":null},"thoughtSignature":"sig"}`},
		{"call args array", `{"functionCall":{"name":"f","args":[]},"thoughtSignature":"sig"}`},
		{"call args string", `{"functionCall":{"name":"f","args":"{}"},"thoughtSignature":"sig"}`},
		{"call args null", `{"functionCall":{"name":"f","args":null},"thoughtSignature":"sig"}`},
		{"call name number", `{"functionCall":{"name":42},"thoughtSignature":"sig"}`},
		{"call name empty", `{"functionCall":{"name":""},"thoughtSignature":"sig"}`},
		{"call name null", `{"functionCall":{"name":null},"thoughtSignature":"sig"}`},
		{"call name missing", `{"functionCall":{"args":{}},"thoughtSignature":"sig"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("output", func(t *testing.T) {
				raw := `{"candidates":[{"content":{"parts":[` + tc.part + `]},"finishReason":"STOP"}]}`
				response, err := nativeGeminiResponse([]byte(raw), "gemini", "resp_test")
				require.Error(t, err)
				require.Nil(t, response)
			})
			t.Run("replay", func(t *testing.T) {
				envelope := geminiResponsesEnvelope + base64.RawURLEncoding.EncodeToString([]byte(`{"call_id":"c","part":`+tc.part+`}`))
				var part map[string]json.RawMessage
				require.NoError(t, json.Unmarshal([]byte(tc.part), &part))
				paired := map[string]any{"type": "message", "role": "assistant", "id": "c", "content": "answer"}
				if _, exists := part["functionCall"]; exists {
					paired = map[string]any{"type": "function_call", "call_id": "c", "name": "f", "arguments": "{}"}
				}
				body, err := json.Marshal(map[string]any{"model": "gemini", "input": []any{
					map[string]any{"type": "reasoning", "encrypted_content": envelope}, paired,
				}})
				require.NoError(t, err)
				payload, req, err := responsesToNativeGemini(body)
				require.Error(t, err)
				require.Nil(t, payload)
				require.Nil(t, req)
			})
		})
	}
}

func TestNativeGeminiResponsesValidSignedPartReplay(t *testing.T) {
	for _, part := range []string{
		`{"text":"","thought":false,"thoughtSignature":"sig"}`,
		`{"text":"thought","thought":true,"thoughtSignature":"sig"}`,
		`{"functionCall":{"name":"f"},"thoughtSignature":"sig"}`,
		`{"functionCall":{"id":"c","name":"f","args":{"n":9007199254740993}},"thought":true,"thoughtSignature":"sig"}`,
	} {
		t.Run(part, func(t *testing.T) {
			raw := `{"candidates":[{"content":{"parts":[` + part + `]},"finishReason":"STOP"}]}`
			response, err := nativeGeminiResponse([]byte(raw), "gemini", "resp_test")
			require.NoError(t, err)
			require.Equal(t, "completed", response.Status)
			body, err := json.Marshal(map[string]any{"model": "gemini", "input": response.Output})
			require.NoError(t, err)
			payload, _, err := responsesToNativeGemini(body)
			require.NoError(t, err)
			var decoded map[string]any
			require.NoError(t, decodeOpenAIJSONUseNumber(payload, &decoded))
			var original map[string]any
			require.NoError(t, decodeOpenAIJSONUseNumber([]byte(part), &original))
			contents, ok := decoded["contents"].([]any)
			require.True(t, ok)
			require.Len(t, contents, 1)
			content, ok := contents[0].(map[string]any)
			require.True(t, ok)
			parts, ok := content["parts"].([]any)
			require.True(t, ok)
			require.Equal(t, []any{original}, parts)
		})
	}
}
