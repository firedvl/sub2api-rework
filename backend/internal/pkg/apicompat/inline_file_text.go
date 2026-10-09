package apicompat

import (
	"bytes"
	"encoding/json"
	"strings"
)

// InlineFileText returns only the valid plain text the native bridge forwards.
// Invalid data and binary documents remain opaque to text-only policy checks.
func InlineFileText(fileData string) string {
	source := dataURIToAnthropicImageSource(fileData)
	if source == nil || source.MediaType != "text/plain" {
		return ""
	}
	source, err := dataURIToAnthropicFileSource(fileData)
	if err != nil {
		return ""
	}
	return source.Data
}

// InlineFilePartText selects file fields with the bridge's typed JSON semantics.
func InlineFilePartText(raw []byte) string {
	var kind struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return ""
	}
	switch kind.Type {
	case "input_file":
		var part ResponsesContentPart
		if json.Unmarshal(raw, &part) == nil {
			return InlineFileText(part.FileData)
		}
	case "file":
		var part ChatContentPart
		if json.Unmarshal(raw, &part) == nil && part.File != nil {
			return InlineFileText(part.File.FileData)
		}
	}
	return ""
}

// NormalizeInlineFilePartsForInspection makes a guard-only copy; forwarding
// retains the original body. Raw parts preserve duplicate/null field semantics.
func NormalizeInlineFilePartsForInspection(body []byte) ([]byte, error) {
	return normalizeInlineFileRequest(body, true)
}

func normalizeInlineFileRequest(raw []byte, envelope bool) ([]byte, error) {
	var fields struct {
		Messages json.RawMessage `json:"messages"`
		Input    json.RawMessage `json:"input"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	for name, value := range map[string]json.RawMessage{"messages": fields.Messages, "input": fields.Input} {
		if len(value) != 0 {
			setInspectionField(object, name, normalizeInlineFileMessages(value, name == "messages"))
		}
	}
	if envelope && len(fields.Response) != 0 && bytes.HasPrefix(bytes.TrimSpace(fields.Response), []byte("{")) {
		response, err := normalizeInlineFileRequest(fields.Response, false)
		if err != nil {
			return nil, err
		}
		setInspectionField(object, "response", response)
	}
	return json.Marshal(object)
}

func setInspectionField(object map[string]json.RawMessage, name string, value json.RawMessage) {
	for field := range object {
		if strings.EqualFold(field, name) {
			delete(object, field)
		}
	}
	object[name] = value
}

func normalizeInlineFileMessages(raw json.RawMessage, chat bool) json.RawMessage {
	var messages []json.RawMessage
	if json.Unmarshal(raw, &messages) != nil {
		return raw
	}
	for i, message := range messages {
		var fields struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(message, &fields) != nil || json.Unmarshal(message, &object) != nil || object == nil {
			continue
		}
		var parts []json.RawMessage
		if json.Unmarshal(fields.Content, &parts) != nil {
			continue
		}
		hasInlineText := false
		for j, part := range parts {
			if text := InlineFilePartText(part); text != "" {
				hasInlineText = true
				parts[j], _ = json.Marshal(struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{Type: "text", Text: text})
			}
		}
		content, _ := json.Marshal(parts)
		setInspectionField(object, "content", content)
		if hasInlineText && inlineFileUsesUserFallback(fields.Role, fields.Type, chat) {
			fields.Role = "user"
		}
		role, _ := json.Marshal(fields.Role)
		setInspectionField(object, "role", role)
		messages[i], _ = json.Marshal(object)
	}
	result, _ := json.Marshal(messages)
	return result
}

func inlineFileUsesUserFallback(role, kind string, chat bool) bool {
	if chat {
		switch role {
		case "system", "user", "assistant", "tool", "function":
			return false
		}
		return true
	}
	if role == "system" || role == "developer" {
		return false
	}
	switch kind {
	case "function_call", "function_call_output", "reasoning":
		return false
	}
	return role != "user" && role != "assistant"
}
