package service

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	gatewayFeatureSupported   = "supported"
	gatewayFeatureUnsupported = "unsupported"
	gatewayFeatureConditional = "conditional"
	gatewayFeatureUnknown     = "unknown"
)

var gatewayFeatureKeys = []string{
	"protocol", "text_input", "image_input", "audio_input", "video_input", "streaming",
	"functions", "web_search", "tool_discovery", "reasoning",
	"service_tier.priority", "service_tier.flex", "service_tier.ultrafast",
}

type GatewayFeatureSet struct {
	Features         map[string]string             `json:"features"`
	Constraints      []GatewayCapabilityConstraint `json:"constraints"`
	ReasoningEfforts []string                      `json:"reasoning_efforts,omitempty"`
	ContextWindow    int64                         `json:"context_window,omitempty"`
}

type GatewayCapabilityConstraint struct {
	AllOf  []string `json:"all_of"`
	Reason string   `json:"reason"`
}

// gatewayAccountFeatures reports only account-scoped observations. A mapping
// is routing configuration, not evidence that the upstream model has a feature.
func gatewayAccountFeatures(account *Account, upstreamModel, protocol string) GatewayFeatureSet {
	return gatewayAccountFeaturesWithMetadata(account, upstreamModel, protocol, gatewayAccountMetadata(account))
}

// gatewayAccountFeaturesWithMetadata reports only account-scoped observations.
// A mapping is routing configuration, not evidence that the upstream model has
// a feature. Callers evaluating several models for one account should reuse
// gatewayAccountMetadata rather than decoding its snapshots repeatedly.
func gatewayAccountFeaturesWithMetadata(account *Account, upstreamModel, protocol string, metadata map[string]UpstreamModelMetadata) GatewayFeatureSet {
	result := gatewayUnknownFeatures()
	if account == nil || strings.TrimSpace(upstreamModel) == "" {
		return result
	}
	if gatewayProtocolFeature(account, protocol) == gatewayFeatureUnsupported {
		result.Features["protocol"] = gatewayFeatureUnsupported
		return result
	}
	if account.Platform == PlatformOpenAI && protocol == CompositeRouteEndpointResponses && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVisionInput) {
		result.Features["image_input"] = gatewayFeatureUnsupported
	}
	if account.Platform == PlatformAntigravity {
		result.Features["protocol"] = gatewayFeatureSupported
		result.Features["functions"] = gatewayFeatureSupported
		result.Features["web_search"] = gatewayFeatureSupported
		result.Constraints = []GatewayCapabilityConstraint{{
			AllOf:  []string{"functions", "web_search"},
			Reason: "CAPABILITY_COMBINATION_UNSUPPORTED",
		}}
	}

	modelMetadata, known := metadata[strings.TrimSpace(upstreamModel)]
	if !known {
		return result
	}
	if value, declared := gatewayCodexCapabilityBool(modelMetadata.CodexToolCapabilities, "supported_in_api"); declared && !value {
		result.Features["protocol"] = gatewayFeatureUnsupported
		return result
	}

	result.Features["protocol"] = gatewayProtocolFeature(account, protocol)
	result.Features["streaming"] = result.Features["protocol"]
	if gatewayMetadataHasModality(modelMetadata, "text") {
		result.Features["text_input"] = gatewayFeatureSupported
	}
	if gatewayMetadataHasModality(modelMetadata, "image") {
		result.Features["image_input"] = gatewayFeatureSupported
	}
	if account.Platform == PlatformOpenAI && result.Features["image_input"] == gatewayFeatureSupported &&
		!account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVisionInput) {
		result.Features["image_input"] = gatewayFeatureUnknown
		if protocol == CompositeRouteEndpointResponses {
			result.Features["image_input"] = gatewayFeatureUnsupported
		}
	}
	if modelMetadata.Reasoning != nil {
		result.Features["reasoning"] = gatewayFeatureUnsupported
		if *modelMetadata.Reasoning {
			result.Features["reasoning"] = gatewayFeatureSupported
		}
	}
	result.ReasoningEfforts = normalizeReasoningLevels(modelMetadata.SupportedReasoningLevels)
	result.ContextWindow = modelMetadata.ContextWindow

	if value, declared := gatewayCodexCapabilityBool(modelMetadata.CodexToolCapabilities, "supports_search_tool"); declared {
		result.Features["tool_discovery"] = gatewayFeatureUnsupported
		if value {
			result.Features["tool_discovery"] = gatewayFeatureSupported
		}
	}
	return result
}

func gatewayUnknownFeatures() GatewayFeatureSet {
	features := make(map[string]string, len(gatewayFeatureKeys))
	for _, key := range gatewayFeatureKeys {
		features[key] = gatewayFeatureUnknown
	}
	return GatewayFeatureSet{Features: features, Constraints: []GatewayCapabilityConstraint{}}
}

// GatewayChatAccountCompatible mirrors the final account gate in Chat
// Completions: Gemini chat uses its native account path, not mixed scheduling.
func GatewayChatAccountCompatible(targetPlatform string, account *Account) bool {
	return account != nil && (targetPlatform != PlatformGemini || account.Platform == PlatformGemini)
}

func gatewayProtocolFeature(account *Account, protocol string) string {
	switch protocol {
	case CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointMessages:
	default:
		return gatewayFeatureUnknown
	}
	// Text Responses and Messages use the same chat eligibility predicate as
	// Chat Completions; native Responses is only required for special shapes.
	if account.IsOpenAICompatible() {
		if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
			return gatewayFeatureUnsupported
		}
		return gatewayFeatureSupported
	}
	if account.Platform == PlatformGemini && protocol == CompositeRouteEndpointResponses {
		return gatewayFeatureUnknown
	}
	if account.Platform == PlatformAnthropic || account.Platform == PlatformGemini || account.Platform == PlatformAntigravity {
		return gatewayFeatureSupported
	}
	return gatewayFeatureUnknown
}

func gatewayMetadataHasModality(metadata UpstreamModelMetadata, wanted string) bool {
	for _, modality := range metadata.InputModalities {
		if strings.EqualFold(strings.TrimSpace(modality), wanted) {
			return true
		}
	}
	return false
}

func gatewayCodexCapabilityBool(capabilities map[string]json.RawMessage, key string) (bool, bool) {
	if capabilities == nil {
		return false, false
	}
	raw := bytes.TrimSpace(capabilities[key])
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false, false
	}
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return value, true
}

// gatewayAccountMetadata combines durable metadata with one fresh manifest
// from the same account identity. The manifest is dynamic provider evidence;
// generated local descriptors are deliberately excluded.
func gatewayAccountMetadata(account *Account) map[string]UpstreamModelMetadata {
	return gatewayAccountMetadataWithManifest(account, observeOpenAICodexManifest(account, time.Now()))
}

func gatewayAccountMetadataWithManifest(account *Account, observation codexManifestObservation) map[string]UpstreamModelMetadata {
	if account == nil {
		return nil
	}
	metadata := make(map[string]UpstreamModelMetadata)
	if snapshot := account.GetUpstreamModelMetadataSnapshot(); snapshot != nil {
		for modelID, entry := range snapshot.Models {
			metadata[strings.TrimSpace(modelID)] = entry
		}
	}
	for modelID, entry := range observation.metadata {
		if durable, found := metadata[modelID]; found {
			entry = gatewayMergeUpstreamModelMetadata(entry, durable)
		}
		metadata[modelID] = entry
	}
	return metadata
}

func gatewayMergeUpstreamModelMetadata(primary, fallback UpstreamModelMetadata) UpstreamModelMetadata {
	merged, _ := mergeUpstreamModelMetadata(primary, fallback)
	if len(primary.CodexToolCapabilities) == 0 && len(fallback.CodexToolCapabilities) == 0 {
		return merged
	}
	merged.CodexToolCapabilities = make(map[string]json.RawMessage, len(primary.CodexToolCapabilities)+len(fallback.CodexToolCapabilities))
	for key, value := range fallback.CodexToolCapabilities {
		merged.CodexToolCapabilities[key] = append(json.RawMessage(nil), value...)
	}
	for key, value := range primary.CodexToolCapabilities {
		merged.CodexToolCapabilities[key] = append(json.RawMessage(nil), value...)
	}
	return merged
}

// codexManifestObservation is owned by one request. Both consumers use the
// same selected, freshness-checked and decoded body.
type codexManifestObservation struct {
	metadata  map[string]UpstreamModelMetadata
	publicIDs []string
}

type codexManifestModel struct {
	Visibility               string          `json:"visibility"`
	Slug                     string          `json:"slug"`
	ID                       string          `json:"id"`
	SupportedInAPI           *bool           `json:"supported_in_api"`
	Reasoning                *bool           `json:"reasoning"`
	SupportedReasoningLevels []string        `json:"supported_reasoning_levels"`
	InputModalities          []string        `json:"input_modalities"`
	ContextWindow            int64           `json:"context_window"`
	SupportsSearchTool       json.RawMessage `json:"supports_search_tool"`
}

// Decode the complete JSON once. Entry validation below preserves the old
// distinction: malformed metadata drops that entry, while malformed publication
// fields invalidate the selected body's public list (never select an older body).
func parseCodexManifestObservation(body []byte) (codexManifestObservation, bool) {
	// Invalid candidates are scanned, not decoded, so fallback never decodes
	// more than the one selected manifest body.
	if !gjson.ValidBytes(body) {
		return codexManifestObservation{}, false
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return codexManifestObservation{}, false
	}
	var modelArray gjson.Result
	root.ForEach(func(key, value gjson.Result) bool {
		if key.String() == "models" {
			modelArray = value
		}
		return true
	})
	if !modelArray.IsArray() {
		return codexManifestObservation{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var envelope map[string]any
	if decoder.Decode(&envelope) != nil {
		return codexManifestObservation{}, false
	}
	models, ok := envelope["models"].([]any)
	if !ok {
		return codexManifestObservation{}, false
	}

	metadata := make(map[string]UpstreamModelMetadata, len(models))
	publicIDs := make(map[string]struct{})
	publicationValid := true
	for _, raw := range models {
		entry, metadataOK, publicOK := codexManifestEntry(raw)
		publicationValid = publicationValid && publicOK
		slug, visibility := strings.TrimSpace(entry.Slug), strings.TrimSpace(entry.Visibility)
		if slug != "" && (visibility == "" || visibility == "list") &&
			(entry.SupportedInAPI == nil || *entry.SupportedInAPI) &&
			!strings.Contains(slug, "*") && !strings.HasPrefix(slug, codexAutoModelPrefix) && !isCodexDedicatedMediaModel(slug) {
			publicIDs[slug] = struct{}{}
		}
		if !metadataOK {
			continue
		}
		modelID := strings.TrimSpace(entry.Slug)
		if modelID == "" {
			modelID = strings.TrimSpace(entry.ID)
		}
		if modelID == "" {
			continue
		}
		manifestMetadata := UpstreamModelMetadata{ID: modelID, Reasoning: entry.Reasoning, InputModalities: normalizeCodexInputModalities(entry.InputModalities), ContextWindow: entry.ContextWindow}
		manifestMetadata.SupportedReasoningLevels = normalizeReasoningLevels(entry.SupportedReasoningLevels)
		if entry.SupportedInAPI != nil {
			manifestMetadata.CodexToolCapabilities = map[string]json.RawMessage{"supported_in_api": []byte(strconv.FormatBool(*entry.SupportedInAPI))}
		}
		if len(bytes.TrimSpace(entry.SupportsSearchTool)) > 0 {
			if manifestMetadata.CodexToolCapabilities == nil {
				manifestMetadata.CodexToolCapabilities = make(map[string]json.RawMessage)
			}
			manifestMetadata.CodexToolCapabilities["supports_search_tool"] = append(json.RawMessage(nil), entry.SupportsSearchTool...)
		}
		if _, found := metadata[modelID]; !found {
			metadata[modelID] = manifestMetadata
		}
		if alternateID := strings.TrimSpace(entry.ID); alternateID != "" {
			if _, found := metadata[alternateID]; !found {
				metadata[alternateID] = manifestMetadata
			}
		}
	}
	if !publicationValid {
		publicIDs = nil
	}
	return codexManifestObservation{metadata: metadata, publicIDs: sortedModelIDSet(publicIDs)}, true
}

func codexManifestEntry(raw any) (codexManifestModel, bool, bool) {
	var entry codexManifestModel
	if raw == nil {
		return entry, true, true
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return entry, false, false
	}
	fields = codexManifestFoldFields(fields)
	stringField := func(key string, target *string) bool {
		if fields[key] == nil {
			return true
		}
		value, ok := fields[key].(string)
		*target = value
		return ok
	}
	boolField := func(key string, target **bool) bool {
		if fields[key] == nil {
			return true
		}
		value, ok := fields[key].(bool)
		if ok {
			*target = &value
		}
		return ok
	}
	slugOK := stringField("slug", &entry.Slug)
	apiOK := boolField("supported_in_api", &entry.SupportedInAPI)
	publicOK := stringField("visibility", &entry.Visibility) && slugOK && apiOK
	// Visibility was not part of the old metadata decoder.
	metadataOK := stringField("id", &entry.ID) && slugOK && apiOK
	metadataOK = boolField("reasoning", &entry.Reasoning) && metadataOK

	if fields["context_window"] != nil {
		value, ok := fields["context_window"].(json.Number)
		if ok {
			var err error
			entry.ContextWindow, err = value.Int64()
			metadataOK = metadataOK && err == nil
		} else {
			metadataOK = false
		}
	}
	if fields["input_modalities"] != nil {
		values, ok := fields["input_modalities"].([]any)
		metadataOK = metadataOK && ok
		for _, raw := range values {
			value, ok := raw.(string)
			metadataOK = metadataOK && (ok || raw == nil)
			entry.InputModalities = append(entry.InputModalities, value)
		}
	}
	if fields["supported_reasoning_levels"] != nil {
		values, ok := fields["supported_reasoning_levels"].([]any)
		metadataOK = metadataOK && ok
		for _, raw := range values {
			if value, ok := raw.(string); ok {
				entry.SupportedReasoningLevels = append(entry.SupportedReasoningLevels, value)
			} else if object, ok := raw.(map[string]any); ok {
				if value, ok := codexManifestFoldFields(object)["effort"].(string); ok {
					entry.SupportedReasoningLevels = append(entry.SupportedReasoningLevels, value)
				}
			}
		}
	}
	if raw, exists := fields["supports_search_tool"]; exists {
		entry.SupportsSearchTool, _ = json.Marshal(raw)
	}
	return entry, metadataOK, publicOK
}

func codexManifestFoldFields(fields map[string]any) map[string]any {
	// The former typed decoder matched field names case-insensitively after
	// canonical key sorting. Keep that behavior without decoding an entry again.
	for key := range fields {
		if key != strings.ToLower(key) {
			keys := make([]string, 0, len(fields))
			for name := range fields {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			normalized := make(map[string]any, len(fields))
			for _, name := range keys {
				normalized[strings.ToLower(name)] = fields[name]
			}
			fields = normalized
			break
		}
	}
	return fields
}

func gatewayIntersectFeatures(sets []GatewayFeatureSet) GatewayFeatureSet {
	result := gatewayUnknownFeatures()
	if len(sets) == 0 {
		return result
	}
	for _, key := range gatewayFeatureKeys {
		states := make(map[string]bool, len(sets))
		for _, set := range sets {
			state := set.Features[key]
			if state == "" {
				state = gatewayFeatureUnknown
			}
			states[state] = true
		}
		switch {
		case states[gatewayFeatureUnknown]:
			result.Features[key] = gatewayFeatureUnknown
		case states[gatewayFeatureSupported] && states[gatewayFeatureUnsupported]:
			result.Features[key] = gatewayFeatureConditional
		case states[gatewayFeatureConditional]:
			result.Features[key] = gatewayFeatureConditional
		case states[gatewayFeatureSupported]:
			result.Features[key] = gatewayFeatureSupported
		case states[gatewayFeatureUnsupported]:
			result.Features[key] = gatewayFeatureUnsupported
		}
	}
	result.Constraints = gatewayUnionConstraints(sets)
	result.ReasoningEfforts = gatewayIntersectReasoningEfforts(sets)
	result.ContextWindow = gatewayMinimumKnownContextWindow(sets)
	return result
}

func gatewayFeatureConstraintMatches(constraint GatewayCapabilityConstraint, features map[string]bool) bool {
	if len(constraint.AllOf) == 0 {
		return false
	}
	for _, key := range constraint.AllOf {
		if !features[key] {
			return false
		}
	}
	return true
}

func gatewayUnionConstraints(sets []GatewayFeatureSet) []GatewayCapabilityConstraint {
	seen := make(map[string]struct{})
	result := []GatewayCapabilityConstraint{}
	for _, set := range sets {
		for _, constraint := range set.Constraints {
			allOf := append([]string(nil), constraint.AllOf...)
			sort.Strings(allOf)
			key := strings.Join(allOf, "\x00") + "\x01" + constraint.Reason
			if len(allOf) == 0 {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, GatewayCapabilityConstraint{AllOf: allOf, Reason: constraint.Reason})
		}
	}
	return result
}

func gatewayIntersectReasoningEfforts(sets []GatewayFeatureSet) []string {
	var common map[string]struct{}
	for _, set := range sets {
		if len(set.ReasoningEfforts) == 0 {
			return nil
		}
		current := make(map[string]struct{}, len(set.ReasoningEfforts))
		for _, effort := range set.ReasoningEfforts {
			if effort = strings.TrimSpace(effort); effort != "" {
				current[effort] = struct{}{}
			}
		}
		if common == nil {
			common = current
			continue
		}
		for effort := range common {
			if _, ok := current[effort]; !ok {
				delete(common, effort)
			}
		}
	}
	result := make([]string, 0, len(common))
	for effort := range common {
		result = append(result, effort)
	}
	sort.Strings(result)
	return result
}

func gatewayMinimumKnownContextWindow(sets []GatewayFeatureSet) int64 {
	var result int64
	for _, set := range sets {
		if set.ContextWindow <= 0 {
			return 0
		}
		if result > 0 && set.ContextWindow >= result {
			continue
		}
		result = set.ContextWindow
	}
	return result
}
