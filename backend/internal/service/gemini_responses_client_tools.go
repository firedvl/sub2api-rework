package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// Validate before flattening: the shared adapter skips unsupported children
// and relaxes namespace-only tool choice, which this native subset must reject.
func adaptNativeGeminiNamespaces(body []byte) ([]byte, apicompat.ResponsesClientToolMapping, error) {
	var request map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &request); err != nil {
		return nil, apicompat.ResponsesClientToolMapping{}, err
	}
	namespaces := map[string]bool{}
	tools, _ := request["tools"].([]any)
	for _, value := range tools {
		tool, ok := value.(map[string]any)
		if kind, ok := tool["type"].(string); ok && strings.TrimSpace(kind) != kind {
			return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("tool type must be canonical")
		}
		if !ok || tool["type"] != "namespace" {
			continue
		}
		raw, _ := json.Marshal(tool)
		if err := nativeGeminiAllowedFields(raw, "type", "name", "description", "tools"); err != nil {
			return nil, apicompat.ResponsesClientToolMapping{}, err
		}
		name, _ := tool["name"].(string)
		children, ok := tool["tools"].([]any)
		if name == "" || strings.TrimSpace(name) != name || namespaces[name] || !ok || len(children) == 0 {
			return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("gemini namespaces require unique names and function children")
		}
		if description, exists := tool["description"]; exists {
			if _, ok := description.(string); !ok {
				return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("namespace description must be text")
			}
		}
		namespaces[name] = true
		seen := map[string]bool{}
		for _, value := range children {
			child, ok := value.(map[string]any)
			childName, _ := child["name"].(string)
			if !ok || child["type"] != "function" || childName == "" || strings.TrimSpace(childName) != childName || seen[childName] {
				return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("gemini namespaces require unique function children")
			}
			seen[childName] = true
			raw, _ := json.Marshal(child)
			if err := nativeGeminiAllowedFields(raw, "type", "name", "description", "parameters", "strict"); err != nil {
				return nil, apicompat.ResponsesClientToolMapping{}, err
			}
			if description, _ := tool["description"].(string); description != "" {
				childDescription, ok := child["description"].(string)
				if value := child["description"]; value != nil && !ok {
					return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("function description must be text")
				}
				child["description"] = name + ": " + description + "\n" + childDescription
			}
		}
	}
	if choice, ok := request["tool_choice"].(map[string]any); ok {
		raw, _ := json.Marshal(choice)
		if err := nativeGeminiAllowedFields(raw, "type", "name", "namespace"); err != nil {
			return nil, apicompat.ResponsesClientToolMapping{}, err
		}
		if kind, _ := choice["type"].(string); strings.TrimSpace(kind) == "namespace" {
			return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("native Gemini Responses does not support namespace-only tool_choice")
		}
		if _, qualified := choice["namespace"]; qualified && len(namespaces) == 0 {
			return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("unknown Gemini tool_choice namespace")
		}
	}
	if len(namespaces) == 0 {
		return body, apicompat.ResponsesClientToolMapping{}, nil
	}
	names, _, err := apicompat.FlattenResponsesNamespaces(request)
	if err != nil {
		return nil, apicompat.ResponsesClientToolMapping{}, err
	}
	if choice, ok := request["tool_choice"].(map[string]any); ok {
		if _, qualified := choice["namespace"]; qualified {
			return nil, apicompat.ResponsesClientToolMapping{}, fmt.Errorf("unknown Gemini tool_choice namespace or function")
		}
	}
	adapted, err := json.Marshal(request)
	return adapted, apicompat.ResponsesClientToolMapping{NamespaceTools: names}, err
}
