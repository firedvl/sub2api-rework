package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// Provider observations stay intact; selectable efforts must pass request validation.
func qualifiedOpenAIReasoningLevels(model string, levels []string) []string {
	qualified := make([]string, 0, len(levels))
	for _, level := range normalizeReasoningLevels(levels) {
		if openai.ValidateGPT61SolReasoningEffort(model, level) == nil {
			qualified = append(qualified, level)
		}
	}
	return qualified
}

func qualifyCodexModelReasoning(raw json.RawMessage, modelID string) (json.RawMessage, error) {
	if openai.ValidateGPT61SolReasoningEffort(modelID, "ultra") == nil {
		return raw, nil
	}
	var model map[string]json.RawMessage
	if err := json.Unmarshal(raw, &model); err != nil {
		return nil, err
	}
	request, _ := json.Marshal(map[string]json.RawMessage{"model": model["slug"]})
	modelEffortValid := validateGPT61SolCompatRequest(request, modelID) == nil
	levelsRaw, exists := model["supported_reasoning_levels"]
	if !exists {
		if defaultRaw, declared := model["default_reasoning_level"]; declared {
			var effort string
			if !modelEffortValid || json.Unmarshal(defaultRaw, &effort) != nil || openai.ValidateGPT61SolReasoningEffort(modelID, effort) != nil {
				delete(model, "default_reasoning_level")
				return json.Marshal(model)
			}
		}
		return raw, nil
	}
	var levels []json.RawMessage
	if err := json.Unmarshal(levelsRaw, &levels); err != nil {
		return nil, fmt.Errorf("decode reasoning levels for %q: %w", modelID, err)
	}
	qualified := make([]json.RawMessage, 0, len(levels))
	efforts := make([]string, 0, len(levels))
	for _, level := range levels {
		var entry struct {
			Effort string `json:"effort"`
		}
		if json.Unmarshal(level, &entry) != nil {
			_ = json.Unmarshal(level, &entry.Effort)
		}
		if !modelEffortValid || strings.TrimSpace(entry.Effort) == "" || openai.ValidateGPT61SolReasoningEffort(modelID, entry.Effort) != nil {
			continue
		}
		qualified = append(qualified, level)
		efforts = append(efforts, entry.Effort)
	}
	changed := len(qualified) != len(levels)
	if defaultRaw, exists := model["default_reasoning_level"]; exists {
		var effort string
		if json.Unmarshal(defaultRaw, &effort) != nil || !stringSliceContains(efforts, effort) {
			delete(model, "default_reasoning_level")
			if len(efforts) > 0 {
				model["default_reasoning_level"], _ = json.Marshal(efforts[0])
			}
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	model["supported_reasoning_levels"], _ = json.Marshal(qualified)
	return json.Marshal(model)
}

// Run on client copies, after snapshot completion and before public alias rewriting.
func qualifyCodexManifestReasoning(manifest *CodexModelsManifest, targets map[string][]string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(manifest.Body, &envelope); err != nil {
		return err
	}
	var models []json.RawMessage
	if err := json.Unmarshal(envelope["models"], &models); err != nil {
		return err
	}
	changed := false
	for i, raw := range models {
		var model struct {
			Slug string `json:"slug"`
		}
		if json.Unmarshal(raw, &model) != nil {
			continue
		}
		model.Slug = strings.TrimSpace(model.Slug)
		modelTargets := targets[model.Slug]
		if targets == nil {
			modelTargets = []string{model.Slug}
		}
		qualified := raw
		for _, target := range modelTargets {
			var err error
			qualified, err = qualifyCodexModelReasoning(qualified, target)
			if err != nil {
				return err
			}
		}
		changed = changed || !bytes.Equal(raw, qualified)
		models[i] = qualified
	}
	if !changed {
		return nil
	}
	envelope["models"], _ = json.Marshal(models)
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	manifest.Body = body
	manifest.ETag = codexModelsManifestBodyETag(body)
	return nil
}

func qualifyGroupCodexManifestReasoning(manifest *CodexModelsManifest, group *Group, accounts []Account, routes []CompositeModelRoute, routesAvailable bool) error {
	modelIDs, err := CodexModelsManifestModelIDs(manifest.Body)
	if err != nil {
		return err
	}
	targets := make(map[string][]string, len(modelIDs))
	for _, modelID := range modelIDs {
		metadata, _ := groupCodexModelMetadata(group.Platform, modelID, accounts, group, routes, routesAvailable)
		targets[modelID] = metadata.reasoningContractModels
	}
	return qualifyCodexManifestReasoning(manifest, targets)
}
