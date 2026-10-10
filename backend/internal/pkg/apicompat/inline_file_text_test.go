package apicompat

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInlineFileTextMatchesBridge(t *testing.T) {
	for _, tc := range []struct {
		name, data, want string
	}{
		{"text", "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("  plain text\n")), "  plain text\n"},
		{"unicode", "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("plain \u4e2d\u6587")), "plain \u4e2d\u6587"},
		{"line breaks", "data:text/plain;base64,cGxh\naW4=", "plain"},
		{"PDF", "data:application/pdf;base64,cGxhaW4=", ""},
		{"unsupported media", "data:text/html;base64,cGxhaW4=", ""},
		{"invalid URI", "text/plain;base64,cGxhaW4=", ""},
		{"unsupported charset", "data:text/plain;charset=utf-8;base64,cGxhaW4=", ""},
		{"invalid base64", "data:text/plain;base64,???", ""},
		{"invalid UTF8", "data:text/plain;base64,/w==", ""},
		{"empty", "data:text/plain;base64,", ""},
		{"blank", "data:text/plain;base64,IAo=", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, InlineFileText(tc.data))
			if tc.want != "" {
				source, err := dataURIToAnthropicFileSource(tc.data)
				require.NoError(t, err)
				require.Equal(t, "text", source.Type)
				require.Equal(t, source.Data, tc.want)
			}
		})
	}
}

func TestInlineFilePartTextMatchesTypedBridgeFields(t *testing.T) {
	const blocked = "data:text/plain;base64,YmxvY2tlZCBmaWxl"
	const ordinary = "data:text/plain;base64,b3JkaW5hcnkgZmlsZQ=="
	for _, tc := range []struct {
		name, raw string
		chat      bool
	}{
		{"responses casing", `{"TYPE":"input_file","FILE_DATA":"` + blocked + `"}`, false},
		{"responses duplicate", `{"type":"input_file","file_data":"` + ordinary + `","file_data":"` + blocked + `"}`, false},
		{"responses null", `{"type":"input_file","type":null,"file_data":"` + blocked + `","file_data":null}`, false},
		{"chat casing", `{"TYPE":"file","FILE":{"FILE_DATA":"` + blocked + `"}}`, true},
		{"chat duplicate", `{"type":"file","file":{"file_data":"` + ordinary + `","file_data":"` + blocked + `"}}`, true},
		{"chat null", `{"type":"file","file":{"file_data":"` + blocked + `","file_data":null}}`, true},
		{"chat pointer merge", `{"type":"file","file":{"file_data":"` + blocked + `"},"file":{"filename":"new name"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, "blocked file", InlineFilePartText([]byte(tc.raw)))
			raw := json.RawMessage("[" + tc.raw + "]")
			if tc.chat {
				var parts []ChatContentPart
				require.NoError(t, json.Unmarshal(raw, &parts))
				var err error
				raw, err = json.Marshal(convertChatContentPartsToResponses(parts))
				require.NoError(t, err)
			}
			converted, err := convertResponsesUserToAnthropicContent(raw)
			require.NoError(t, err)
			var blocks []AnthropicContentBlock
			require.NoError(t, json.Unmarshal(converted, &blocks))
			require.Len(t, blocks, 1)
			require.Equal(t, "text", blocks[0].Type)
			require.Equal(t, "blocked file", blocks[0].Text)
		})
	}
}

func TestNormalizeInlineFilePartsOnlyChangesInspectionCopy(t *testing.T) {
	body := []byte(`{"model":"test","messages":[{"role":"user","content":[{"TYPE":"file","FILE":{"FILE_DATA":"data:text/plain;base64,cGxhaW4=","file_data":null}}]}],"metadata":{"type":"input_file","file_data":"data:text/plain;base64,cGxhaW4="}}`)
	original := string(body)
	normalized, err := NormalizeInlineFilePartsForInspection(body)
	require.NoError(t, err)
	require.Equal(t, original, string(body))
	var root struct {
		Messages []struct {
			Content []ChatContentPart `json:"content"`
		} `json:"messages"`
		Metadata ResponsesContentPart `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(normalized, &root))
	require.Equal(t, "text", root.Messages[0].Content[0].Type)
	require.Equal(t, "plain", root.Messages[0].Content[0].Text)
	require.Equal(t, "input_file", root.Metadata.Type)
	plain := []byte(` {"messages":[{"role":"user","content":"ordinary text"}]} `)
	normalized, err = NormalizeInlineFilePartsForInspection(plain)
	require.NoError(t, err)
	require.JSONEq(t, string(plain), string(normalized))
}

func TestNormalizeInlineFilePartsDoesNotTraverseUnsupportedNesting(t *testing.T) {
	body := []byte(`{"input":` + strings.Repeat("[", 100) + `{"type":"input_file","file_data":"data:text/plain;base64,cGxhaW4="}` + strings.Repeat("]", 100) + `}`)
	require.True(t, json.Valid(body))
	normalized, err := NormalizeInlineFilePartsForInspection(body)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(normalized))
}

func TestInlineFileInspectionPreservesKnownRoleAndToolBoundaries(t *testing.T) {
	for _, tc := range []struct {
		field, role, kind, wantRole string
	}{
		{"messages", "other", "", "user"},
		{"messages", "developer", "", "user"},
		{"messages", "assistant", "", "assistant"},
		{"messages", "tool", "", "tool"},
		{"messages", "function", "", "function"},
		{"input", "other", "", "user"},
		{"input", "tool", "", "user"},
		{"input", "developer", "", "developer"},
		{"input", "assistant", "", "assistant"},
		{"input", "other", "function_call", "other"},
		{"input", "other", "function_call_output", "other"},
		{"input", "other", "reasoning", "other"},
	} {
		body, err := json.Marshal(map[string]any{tc.field: []any{map[string]any{
			"role": tc.role, "type": tc.kind, "content": []any{map[string]string{"type": "input_file", "file_data": "data:text/plain;base64,cGxhaW4="}},
		}}})
		require.NoError(t, err)
		normalized, err := NormalizeInlineFilePartsForInspection(body)
		require.NoError(t, err)
		var root map[string][]ResponsesInputItem
		require.NoError(t, json.Unmarshal(normalized, &root))
		require.Equal(t, tc.wantRole, root[tc.field][0].Role)
	}
	plain := []byte(`{"messages":[{"role":"other","content":[{"type":"text","text":"ordinary text"}]}]}`)
	normalized, err := NormalizeInlineFilePartsForInspection(plain)
	require.NoError(t, err)
	require.JSONEq(t, string(plain), string(normalized))
}

func TestInlineFileInspectionIgnoresChatMessageTypeExtension(t *testing.T) {
	for _, extension := range []string{`17`, `{}`, `[]`, `null`} {
		body := []byte(`{"model":"test","messages":[{"role":"user","type":` + extension + `,"content":[{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}}]}]}`)
		var chat ChatCompletionsRequest
		require.NoError(t, json.Unmarshal(body, &chat))
		responses, err := ChatCompletionsToResponses(&chat)
		require.NoError(t, err)
		anthropic, err := ResponsesToAnthropicRequest(responses)
		require.NoError(t, err)
		require.Contains(t, string(anthropic.Messages[0].Content), "blocked file")
		normalized, err := NormalizeInlineFilePartsForInspection(body)
		require.NoError(t, err)
		require.Contains(t, string(normalized), `"text":"blocked file"`)
	}
}
