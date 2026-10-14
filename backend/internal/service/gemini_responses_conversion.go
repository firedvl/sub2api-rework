package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// Gemini signatures are opaque provider data, not gateway credentials. Carry the
// original part in a reasoning envelope so full-history clients can replay it.
const geminiResponsesEnvelope = "gemini_part_v1_"

func responsesToNativeGemini(body []byte) ([]byte, *apicompat.ResponsesRequest, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, nil, err
	}
	allowed := map[string]bool{"model": true, "input": true, "instructions": true, "stream": true, "max_output_tokens": true, "temperature": true, "top_p": true, "tools": true, "tool_choice": true, "store": true, "previous_response_id": true}
	for key := range fields {
		if !allowed[key] {
			return nil, nil, fmt.Errorf("native Gemini Responses does not support %s", key)
		}
	}
	var rawTools []json.RawMessage
	if raw := fields["tools"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawTools); err != nil {
			return nil, nil, fmt.Errorf("tools must be an array")
		}
		for _, tool := range rawTools {
			if err := nativeGeminiAllowedFields(tool, "type", "name", "description", "parameters", "strict"); err != nil {
				return nil, nil, err
			}
		}
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(fields["input"], &rawItems) == nil {
		for _, item := range rawItems {
			if err := nativeGeminiAllowedFields(item, "type", "role", "content", "encrypted_content", "call_id", "name", "arguments", "id", "output", "status", "summary"); err != nil {
				return nil, nil, err
			}
			var object map[string]json.RawMessage
			_ = json.Unmarshal(item, &object)
			var parts []json.RawMessage
			if json.Unmarshal(object["content"], &parts) == nil {
				for _, part := range parts {
					if err := nativeGeminiAllowedFields(part, "type", "text", "annotations"); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	var req apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, nil, fmt.Errorf("model is required")
	}
	if req.PreviousResponseID != "" || (req.Store != nil && *req.Store) {
		return nil, nil, fmt.Errorf("native Gemini Responses requires store=false and full conversation history")
	}
	var items []apicompat.ResponsesInputItem
	var text string
	if json.Unmarshal(req.Input, &text) == nil {
		content, _ := json.Marshal(text)
		items = []apicompat.ResponsesInputItem{{Role: "user", Content: content}}
	} else if err := json.Unmarshal(req.Input, &items); err != nil {
		return nil, nil, fmt.Errorf("input must be text or an item array")
	}
	contents := []map[string]any{}
	system := []map[string]any{}
	if req.Instructions != "" {
		system = append(system, map[string]any{"text": req.Instructions})
	}
	calls := map[string]string{}
	providerIDs := map[string]string{}
	completedCalls := map[string]bool{}
	signed := map[string]map[string]any{}
	seenEnvelopeIDs := map[string]bool{}
	appendPart := func(role string, part map[string]any) {
		if len(contents) > 0 && contents[len(contents)-1]["role"] == role {
			last := contents[len(contents)-1]
			parts, _ := last["parts"].([]map[string]any)
			last["parts"] = append(parts, part)
		} else {
			contents = append(contents, map[string]any{"role": role, "parts": []map[string]any{part}})
		}
	}
	for _, item := range items {
		switch item.Type {
		case "reasoning":
			if !strings.HasPrefix(item.EncryptedContent, geminiResponsesEnvelope) {
				return nil, nil, fmt.Errorf("unsupported reasoning history")
			}
			data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(item.EncryptedContent, geminiResponsesEnvelope))
			if err != nil {
				return nil, nil, fmt.Errorf("invalid Gemini reasoning envelope")
			}
			var envelope struct {
				CallID string         `json:"call_id"`
				Part   map[string]any `json:"part"`
			}
			if decodeOpenAIJSONUseNumber(data, &envelope) != nil || envelope.CallID == "" || envelope.Part == nil {
				return nil, nil, fmt.Errorf("invalid Gemini reasoning envelope")
			}
			if seenEnvelopeIDs[envelope.CallID] {
				return nil, nil, fmt.Errorf("duplicate Gemini reasoning envelope")
			}
			seenEnvelopeIDs[envelope.CallID] = true
			signature, ok := envelope.Part["thoughtSignature"].(string)
			if !ok || signature == "" {
				return nil, nil, fmt.Errorf("invalid Gemini signature")
			}
			for key := range envelope.Part {
				if key != "thoughtSignature" && key != "functionCall" && key != "text" && key != "thought" {
					return nil, nil, fmt.Errorf("unsupported Gemini reasoning envelope")
				}
			}
			_, hasText := envelope.Part["text"].(string)
			_, hasCall := envelope.Part["functionCall"].(map[string]any)
			if hasText == hasCall {
				return nil, nil, fmt.Errorf("invalid Gemini reasoning part")
			}
			if hasText && envelope.Part["thought"] == true {
				appendPart("model", envelope.Part)
			} else {
				if signed[envelope.CallID] != nil {
					return nil, nil, fmt.Errorf("duplicate Gemini reasoning envelope")
				}
				signed[envelope.CallID] = envelope.Part
			}
		case "function_call":
			if item.CallID == "" || item.Name == "" || calls[item.CallID] != "" {
				return nil, nil, fmt.Errorf("function calls require unique call_id and name")
			}
			var args map[string]any
			if decodeOpenAIJSONUseNumber([]byte(item.Arguments), &args) != nil || args == nil {
				return nil, nil, fmt.Errorf("function arguments must be a JSON object")
			}
			part := map[string]any{"functionCall": map[string]any{"id": item.CallID, "name": item.Name, "args": args}}
			if original, ok := signed[item.CallID]; ok {
				fc, ok := original["functionCall"].(map[string]any)
				if !ok || fc["name"] != item.Name {
					return nil, nil, fmt.Errorf("gemini signature does not match function call")
				}
				if providerID, ok := fc["id"].(string); ok && providerID != "" && providerID != item.CallID {
					return nil, nil, fmt.Errorf("gemini signed call ID changed")
				}
				providerIDs[item.CallID], _ = fc["id"].(string)
				originalArgsValue := fc["args"]
				if originalArgsValue == nil {
					originalArgsValue = map[string]any{}
				}
				originalArgs, _ := json.Marshal(originalArgsValue)
				newArgs, _ := json.Marshal(args)
				if string(originalArgs) != string(newArgs) {
					return nil, nil, fmt.Errorf("gemini signed function arguments changed")
				}
				part = original
				delete(signed, item.CallID)
			} else {
				providerIDs[item.CallID] = item.CallID
			}
			calls[item.CallID] = item.Name
			appendPart("model", part)
		case "function_call_output":
			name := calls[item.CallID]
			if name == "" || completedCalls[item.CallID] {
				return nil, nil, fmt.Errorf("function output requires an unmatched preceding call: %s", item.CallID)
			}
			completedCalls[item.CallID] = true
			var value any
			if decodeOpenAIJSONUseNumber([]byte(item.Output), &value) != nil {
				value = item.Output
			}
			response, ok := value.(map[string]any)
			if !ok {
				response = map[string]any{"result": value}
			}
			result := map[string]any{"name": name, "response": response}
			if providerIDs[item.CallID] != "" {
				result["id"] = providerIDs[item.CallID]
			}
			appendPart("user", map[string]any{"functionResponse": result})
		case "", "message":
			role := item.Role
			switch role {
			case "assistant":
				role = "model"
			case "user", "system", "developer":
			default:
				return nil, nil, fmt.Errorf("unsupported message role %q", role)
			}
			var parts []apicompat.ResponsesContentPart
			var text *string
			if json.Unmarshal(item.Content, &text) == nil && text != nil {
				parts = []apicompat.ResponsesContentPart{{Type: "input_text", Text: *text}}
			} else if json.Unmarshal(item.Content, &parts) != nil || parts == nil {
				return nil, nil, fmt.Errorf("invalid text content")
			}
			for _, p := range parts {
				if p.Type != "input_text" && p.Type != "output_text" {
					return nil, nil, fmt.Errorf("native Gemini Responses supports text input only")
				}
				part := map[string]any{"text": p.Text}
				if original, ok := signed[item.ID]; ok {
					if role != "model" || len(parts) != 1 || original["text"] != p.Text {
						return nil, nil, fmt.Errorf("gemini signed message changed")
					}
					part = original
					delete(signed, item.ID)
				}
				if role == "system" || role == "developer" {
					system = append(system, part)
				} else {
					appendPart(role, part)
				}
			}
		default:
			return nil, nil, fmt.Errorf("unsupported input item %q", item.Type)
		}
	}
	if len(signed) > 0 {
		return nil, nil, fmt.Errorf("unpaired Gemini reasoning envelope")
	}
	if len(contents) == 0 {
		return nil, nil, fmt.Errorf("input must contain conversation content")
	}
	out := map[string]any{"contents": contents}
	if len(system) > 0 {
		out["systemInstruction"] = map[string]any{"parts": system}
	}
	gen := map[string]any{}
	if req.MaxOutputTokens != nil {
		if *req.MaxOutputTokens <= 0 {
			return nil, nil, fmt.Errorf("max_output_tokens must be positive")
		}
		gen["maxOutputTokens"] = *req.MaxOutputTokens
	}
	if req.Temperature != nil {
		gen["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		gen["topP"] = *req.TopP
	}
	if len(gen) > 0 {
		out["generationConfig"] = gen
	}
	declarations := []map[string]any{}
	names := map[string]bool{}
	for _, tool := range req.Tools {
		if tool.Type != "function" || tool.Name == "" || (tool.Strict != nil && *tool.Strict) || names[tool.Name] {
			return nil, nil, fmt.Errorf("native Gemini Responses requires unique non-strict function tools")
		}
		names[tool.Name] = true
		declaration := map[string]any{"name": tool.Name, "description": tool.Description}
		if len(tool.Parameters) > 0 {
			var schema map[string]any
			if decodeOpenAIJSONUseNumber(tool.Parameters, &schema) != nil || schema["type"] != "object" {
				return nil, nil, fmt.Errorf("function parameters must be an object schema")
			}
			declaration["parametersJsonSchema"] = schema
		}
		declarations = append(declarations, declaration)
	}
	if len(declarations) > 0 {
		out["tools"] = []any{map[string]any{"functionDeclarations": declarations}}
	}
	if len(req.ToolChoice) > 0 {
		var choice string
		var forced struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}
		config := map[string]any{}
		if json.Unmarshal(req.ToolChoice, &choice) == nil {
			switch choice {
			case "auto":
				config["mode"] = "AUTO"
			case "none":
				config["mode"] = "NONE"
			case "required":
				config["mode"] = "ANY"
			default:
				return nil, nil, fmt.Errorf("unsupported tool_choice")
			}
		} else if json.Unmarshal(req.ToolChoice, &forced) == nil && forced.Type == "function" && names[forced.Name] {
			config["mode"] = "ANY"
			config["allowedFunctionNames"] = []string{forced.Name}
		} else {
			return nil, nil, fmt.Errorf("unsupported tool_choice")
		}
		out["toolConfig"] = map[string]any{"functionCallingConfig": config}
	}
	data, err := json.Marshal(out)
	return data, &req, err
}

func nativeGeminiResponse(raw []byte, model, responseID string) (*apicompat.ResponsesResponse, error) {
	var body map[string]any
	if err := decodeOpenAIJSONUseNumber(raw, &body); err != nil {
		return nil, err
	}
	response := &apicompat.ResponsesResponse{ID: responseID, Object: "response", CreatedAt: time.Now().Unix(), Model: model, Status: "completed", Output: []apicompat.ResponsesOutput{}}
	if body["error"] != nil {
		response.Status = "failed"
		response.Error = &apicompat.ResponsesError{Code: "upstream_error", Message: "Gemini stream returned an error"}
		return response, nil
	}
	candidates, ok := body["candidates"].([]any)
	if body["candidates"] != nil && !ok {
		return nil, fmt.Errorf("invalid Gemini candidates")
	}
	for _, value := range candidates {
		candidate, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Gemini candidate")
		}
		if candidate["content"] == nil {
			continue
		}
		content, ok := candidate["content"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Gemini content")
		}
		if content["parts"] == nil {
			continue
		}
		parts, ok := content["parts"].([]any)
		if !ok {
			return nil, fmt.Errorf("invalid Gemini parts")
		}
		for _, value := range parts {
			part, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid Gemini part")
			}
			_, text := part["text"].(string)
			_, call := part["functionCall"].(map[string]any)
			if text == call {
				return nil, fmt.Errorf("invalid Gemini part union")
			}
			for key := range part {
				if key != "text" && key != "functionCall" && key != "thought" && key != "thoughtSignature" {
					return nil, fmt.Errorf("unsupported Gemini part field")
				}
			}
		}
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("multiple Gemini candidates are unsupported")
	}
	seenCalls := map[string]bool{}
	for i, part := range extractGeminiParts(body) {
		id := fmt.Sprintf("%s_item_%d", responseID, i)
		if value, ok := part["text"].(string); ok {
			if part["thoughtSignature"] != nil {
				envelope, _ := json.Marshal(map[string]any{"call_id": id, "part": part})
				response.Output = append(response.Output, apicompat.ResponsesOutput{Type: "reasoning", ID: id + "_signature", EncryptedContent: geminiResponsesEnvelope + base64.RawURLEncoding.EncodeToString(envelope), Summary: []apicompat.ResponsesSummary{}})
			}
			if part["thought"] == true {
				continue
			}
			response.Output = append(response.Output, apicompat.ResponsesOutput{Type: "message", ID: id, Role: "assistant", Status: "completed", Content: []apicompat.ResponsesContentPart{{Type: "output_text", Text: value}}})
		} else if fc, ok := part["functionCall"].(map[string]any); ok {
			name, _ := fc["name"].(string)
			if name == "" {
				return nil, fmt.Errorf("gemini function call omitted name")
			}
			args := fc["args"]
			if args == nil {
				args = map[string]any{}
			}
			if _, ok := args.(map[string]any); !ok {
				return nil, fmt.Errorf("gemini function arguments must be an object")
			}
			arguments, _ := json.Marshal(args)
			callID, _ := fc["id"].(string)
			if callID == "" {
				callID = id + "_call"
			}
			if seenCalls[callID] {
				return nil, fmt.Errorf("duplicate Gemini function call ID")
			}
			seenCalls[callID] = true
			if part["thoughtSignature"] != nil {
				envelope, _ := json.Marshal(map[string]any{"call_id": callID, "part": part})
				response.Output = append(response.Output, apicompat.ResponsesOutput{Type: "reasoning", ID: id + "_signature", EncryptedContent: geminiResponsesEnvelope + base64.RawURLEncoding.EncodeToString(envelope), Summary: []apicompat.ResponsesSummary{}})
			}
			response.Output = append(response.Output, apicompat.ResponsesOutput{Type: "function_call", ID: id, CallID: callID, Name: name, Arguments: string(arguments), Status: "completed"})
		} else {
			return nil, fmt.Errorf("unsupported Gemini output part")
		}
	}
	finish := extractGeminiFinishReason(body)
	switch finish {
	case "STOP":
		if len(candidates) == 0 {
			return nil, fmt.Errorf("gemini STOP response omitted candidates")
		}
	case "MAX_TOKENS":
		response.Status = "incomplete"
		response.IncompleteDetails = &apicompat.ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY":
		response.Status = "incomplete"
		response.IncompleteDetails = &apicompat.ResponsesIncompleteDetails{Reason: "content_filter"}
	default:
		response.Status = "failed"
		response.Error = &apicompat.ResponsesError{Code: "upstream_error", Message: "Gemini response missing successful termination: " + finish}
	}
	if feedback, ok := body["promptFeedback"].(map[string]any); ok && feedback["blockReason"] != nil {
		response.Status = "incomplete"
		response.Error = nil
		response.IncompleteDetails = &apicompat.ResponsesIncompleteDetails{Reason: "content_filter"}
	}
	response.Usage = nativeGeminiResponsesUsage(body)
	return response, nil
}

func nativeGeminiResponsesUsage(body map[string]any) *apicompat.ResponsesUsage {
	usage, ok := body["usageMetadata"].(map[string]any)
	if !ok {
		return nil
	}
	hasCounts := false
	for _, key := range []string{"promptTokenCount", "candidatesTokenCount", "thoughtsTokenCount", "cachedContentTokenCount"} {
		if value, exists := usage[key]; exists {
			n, valid := asInt(value)
			if !valid || n < 0 {
				return nil
			}
			hasCounts = true
		}
	}
	if !hasCounts {
		return nil
	}
	number := func(key string) int { n, _ := asInt(usage[key]); return max(n, 0) }
	input := number("promptTokenCount")
	thinking := number("thoughtsTokenCount")
	output := number("candidatesTokenCount") + thinking
	return &apicompat.ResponsesUsage{InputTokens: input, OutputTokens: output, TotalTokens: input + output, InputTokensDetails: &apicompat.ResponsesInputTokensDetails{CachedTokens: number("cachedContentTokenCount")}, OutputTokensDetails: &apicompat.ResponsesOutputTokensDetails{ReasoningTokens: thinking}}
}

func nativeGeminiAllowedFields(raw []byte, allowed ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("native Gemini Responses requires JSON objects")
	}
	for key := range object {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("native Gemini Responses does not support field %s", key)
		}
	}
	return nil
}
