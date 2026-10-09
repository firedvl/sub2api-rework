package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func moderationInlineFileBody(t *testing.T, protocol, fileData string) []byte {
	t.Helper()
	field := "input"
	part := map[string]any{"type": "input_file", "file_data": fileData}
	if protocol == ContentModerationProtocolOpenAIChat {
		field = "messages"
		part = map[string]any{"type": "file", "file": map[string]any{"file_data": fileData}}
	}
	body, err := json.Marshal(map[string]any{field: []any{map[string]any{"role": "user", "content": []any{part}}}})
	require.NoError(t, err)
	return body
}

func TestContentModerationInlineTextFiles(t *testing.T) {
	for _, protocol := range []string{ContentModerationProtocolOpenAIChat, ContentModerationProtocolOpenAIResponses} {
		for _, text := range []string{"blocked \u4e2d\u6587", "<system-reminder>blocked text</system-reminder>"} {
			body := moderationInlineFileBody(t, protocol, "data:text/plain;base64,"+base64.StdEncoding.EncodeToString([]byte(text)))
			require.Equal(t, text, extractContentModerationKeywordText(protocol, body))
			if strings.Contains(text, "<system-reminder>") {
				require.Empty(t, ExtractContentModerationText(protocol, body))
			} else {
				require.Equal(t, text, ExtractContentModerationText(protocol, body))
			}
			require.Empty(t, ExtractContentModerationInput(protocol, body).Images)
		}
		for _, data := range []string{"data:application/pdf;base64,cGxhaW4=", "data:text/plain;base64,/w==", "data:text/plain;base64,???", "data:text/plain;base64,IAo="} {
			body := moderationInlineFileBody(t, protocol, data)
			require.Empty(t, ExtractContentModerationText(protocol, body))
			require.Empty(t, extractContentModerationKeywordText(protocol, body))
		}
	}
}

func TestContentModerationInlineFileKeepsLastUserBoundary(t *testing.T) {
	const data = "data:text/plain;base64,YmxvY2tlZCBmaWxl"
	for _, protocol := range []string{ContentModerationProtocolOpenAIChat, ContentModerationProtocolOpenAIResponses} {
		var root map[string]any
		require.NoError(t, json.Unmarshal(moderationInlineFileBody(t, protocol, data), &root))
		for field, messages := range root {
			values, ok := messages.([]any)
			require.True(t, ok)
			root[field] = append(values, map[string]any{"role": "assistant", "content": "next output"})
		}
		body, err := json.Marshal(root)
		require.NoError(t, err)
		require.Empty(t, ExtractContentModerationText(protocol, body))
		require.Empty(t, extractContentModerationKeywordText(protocol, body))
	}
}

func TestContentModerationInlineFileTypedFields(t *testing.T) {
	for _, part := range []string{
		`{"TYPE":"input_file","FILE_DATA":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}`,
		`{"type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnkgZmlsZQ==","file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}`,
		`{"type":"input_file","type":null,"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl","file_data":null}`,
		`{"TYPE":"file","FILE":{"FILE_DATA":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}}`,
		`{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl","file_data":null}}`,
		`{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"},"file":{"filename":"new name"}}`,
	} {
		for _, protocol := range []string{ContentModerationProtocolOpenAIChat, ContentModerationProtocolOpenAIResponses} {
			field := "input"
			if protocol == ContentModerationProtocolOpenAIChat {
				field = "messages"
			}
			body := []byte(`{"` + field + `":[{"role":"user","content":[` + part + `]}]}`)
			require.Equal(t, "blocked file", ExtractContentModerationText(protocol, body))
			require.Equal(t, "blocked file", extractContentModerationKeywordText(protocol, body))
		}
	}
}

func TestContentModerationInlineFileContainers(t *testing.T) {
	const blocked = `[{"type":"input_file","file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}]`
	const ordinary = `[{"type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk="}]`
	for _, message := range []string{
		`{"role":"user","Content":` + blocked + `}`,
		`{"role":"user","content":` + ordinary + `,"content":` + blocked + `}`,
		`{"ROLE":"user","content":` + blocked + `}`,
		`{"role":"other","content":` + blocked + `}`,
		`{"role":"user","type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk=","content":` + blocked + `}`,
	} {
		for _, field := range []string{"input", "INPUT", "messages", "MESSAGES"} {
			protocol := ContentModerationProtocolOpenAIResponses
			if strings.EqualFold(field, "messages") {
				protocol = ContentModerationProtocolOpenAIChat
			}
			body := []byte(`{"TYPE":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk=","` + field + `":[` + message + `]}`)
			require.Equal(t, "blocked file", ExtractContentModerationText(protocol, body))
			require.Equal(t, "blocked file", extractContentModerationKeywordText(protocol, body))
		}
	}
	body := []byte(`{"input":[{"role":"user","content":` + ordinary + `}],"INPUT":[{"role":"user","content":` + blocked + `}]}`)
	require.Equal(t, "blocked file", extractContentModerationKeywordText(ContentModerationProtocolOpenAIResponses, body))
}

func TestContentModerationKeywordBlocksInlineFileOnlyRequests(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModePreBlock
	cfg.KeywordBlockingMode = ContentModerationKeywordModeKeywordOnly
	cfg.BlockedKeywords = []string{"blocked"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	svc := NewContentModerationService(&contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true", SettingKeyContentModerationConfig: string(raw),
	}}, &contentModerationTestRepo{}, nil, nil, nil, nil, nil, nil)
	for _, protocol := range []string{ContentModerationProtocolOpenAIChat, ContentModerationProtocolOpenAIResponses} {
		for _, text := range []string{"blocked file", "ordinary file"} {
			body := moderationInlineFileBody(t, protocol, "data:text/plain;base64,"+base64.StdEncoding.EncodeToString([]byte(text)))
			decision, err := svc.Check(context.Background(), ContentModerationCheckInput{UserID: 1001, Protocol: protocol, Body: body})
			require.NoError(t, err)
			require.Equal(t, text != "ordinary file", decision.Blocked)
			require.Equal(t, text == "ordinary file", decision.Allowed)
		}
	}
}
