package service

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var gatewayEffectiveProtocols = []string{CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointMessages}

// GatewayEffectiveState separates persistent routing decisions from advisory
// scheduler observations. Reason codes never contain provider error text.
type GatewayEffectiveState struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type GatewayEffectiveCatalog struct {
	Published bool   `json:"published"`
	Discovery string `json:"discovery"`
}

type GatewayEffectiveProtocol struct {
	Routing      GatewayEffectiveState  `json:"routing"`
	Availability GatewayEffectiveState  `json:"availability"`
	Capabilities GatewayFeatureSet      `json:"capabilities"`
	Decision     GatewayRoutingDecision `json:"decision"`
}

type GatewayEffectiveModel struct {
	ID        string                              `json:"id"`
	Catalog   GatewayEffectiveCatalog             `json:"catalog"`
	Protocols map[string]GatewayEffectiveProtocol `json:"protocols"`
}

type GatewayEffectiveCapabilities struct {
	SchemaVersion int                     `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	Advisory      bool                    `json:"advisory"`
	CatalogState  string                  `json:"catalog_state"`
	Models        []GatewayEffectiveModel `json:"models"`
}

type GatewayPreflightRequest struct {
	SchemaVersion   int      `json:"schema_version"`
	Model           string   `json:"model"`
	Protocol        string   `json:"protocol"`
	InputModalities []string `json:"input_modalities,omitempty"`
	Tools           []string `json:"tools,omitempty"`
	Streaming       bool     `json:"streaming,omitempty"`
	ServiceTier     string   `json:"service_tier,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
}

func (r GatewayPreflightRequest) Validate() error {
	if r.SchemaVersion != 2 {
		return fmt.Errorf("schema_version must be 2")
	}
	if strings.TrimSpace(r.Model) == "" || len(r.Model) > 256 || strings.ContainsAny(r.Model, "\r\n\t\x00*") {
		return fmt.Errorf("model must be a nonempty public model ID of at most 256 bytes")
	}
	if !slices.Contains(gatewayEffectiveProtocols, r.Protocol) {
		return fmt.Errorf("protocol must be responses, chat_completions, or messages")
	}
	if len(r.InputModalities) > 4 || len(r.Tools) > 3 {
		return fmt.Errorf("too many request traits")
	}
	for _, modality := range r.InputModalities {
		if !slices.Contains([]string{"text", "image", "audio", "video"}, modality) {
			return fmt.Errorf("input_modalities contains an unsupported value")
		}
	}
	for _, tool := range r.Tools {
		if !slices.Contains([]string{"functions", "web_search", "tool_discovery"}, tool) {
			return fmt.Errorf("tools contains an unsupported value")
		}
	}
	if r.ServiceTier != "" && !slices.Contains([]string{"auto", "default", "scale", "priority", "flex", "ultrafast"}, r.ServiceTier) {
		return fmt.Errorf("service_tier contains an unsupported value")
	}
	if r.ReasoningEffort != "" && !slices.Contains([]string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"}, r.ReasoningEffort) {
		return fmt.Errorf("reasoning_effort contains an unsupported value")
	}
	return nil
}

type GatewayPreflightResponse struct {
	SchemaVersion int                     `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	Advisory      bool                    `json:"advisory"`
	Model         string                  `json:"model"`
	Protocol      string                  `json:"protocol"`
	Catalog       GatewayEffectiveCatalog `json:"catalog"`
	Routing       GatewayEffectiveState   `json:"routing"`
	Availability  GatewayEffectiveState   `json:"availability"`
	Support       GatewayEffectiveState   `json:"support"`
	Decision      GatewayRoutingDecision  `json:"decision"`
}

type gatewayEffectiveCandidates struct {
	configured   []GatewayFeatureSet
	current      []GatewayFeatureSet
	observations []GatewayEffectiveState
	reason       string
	view         GatewayEffectiveProtocol
}

// Decode once per account per request; no shared cache or account writes.
func (s *GatewayService) loadGatewayEffectiveSnapshot(ctx context.Context, group *Group) gatewayCapabilitySnapshot {
	now := time.Now()
	return s.loadGatewayEffectiveSnapshotAt(ctx, group, now)
}

func (s *GatewayService) loadGatewayEffectiveSnapshotAt(ctx context.Context, group *Group, now time.Time) gatewayCapabilitySnapshot {
	snapshot := s.loadGatewayCapabilitySnapshot(ctx, group)
	snapshot.observedAt = now
	snapshot.manifestIDs = make(map[int64][]string, len(snapshot.configured))
	snapshot.metadata = make(map[int64]map[string]UpstreamModelMetadata, len(snapshot.configured))
	snapshot.observedModels = make(map[int64]map[string]bool, len(snapshot.configured))
	for i := range snapshot.configured {
		account := &snapshot.configured[i]
		observation := observeOpenAICodexManifest(account, now)
		snapshot.metadata[account.ID] = gatewayAccountMetadataWithManifest(account, observation)
		snapshot.manifestIDs[account.ID] = observation.publicIDs
		observed := make(map[string]bool)
		if inventory := account.GetUpstreamModelInventorySnapshot(); inventory != nil {
			for _, model := range inventory.Models {
				observed[strings.TrimPrefix(strings.TrimSpace(model), "models/")] = true
			}
		}
		for _, model := range observation.publicIDs {
			observed[model] = true
		}
		snapshot.observedModels[account.ID] = observed
	}
	return snapshot
}

// BuildGatewayEffectiveCapabilities reuses durable catalog and passive scheduler
// reads. It does not construct a live manifest or invoke account selection.
func (s *GatewayService) BuildGatewayEffectiveCapabilities(ctx context.Context, group *Group, openAI *OpenAIGatewayService) GatewayEffectiveCapabilities {
	snapshot := s.loadGatewayEffectiveSnapshot(ctx, group)
	result := GatewayEffectiveCapabilities{
		SchemaVersion: 2, GeneratedAt: snapshot.observedAt.UTC().Format(time.RFC3339Nano), Advisory: true,
		CatalogState: "known", Models: []GatewayEffectiveModel{},
	}
	if !snapshot.configuredKnown || !snapshot.routesKnown {
		result.CatalogState = "unknown"
	}
	for _, model := range s.gatewayEffectiveModelIDs(ctx, group, snapshot) {
		entry := GatewayEffectiveModel{ID: model, Catalog: s.gatewayEffectiveCatalog(ctx, group, model, true, snapshot), Protocols: make(map[string]GatewayEffectiveProtocol)}
		for _, protocol := range gatewayEffectiveProtocols {
			candidates := s.gatewayEffectiveCandidates(ctx, group, model, protocol, snapshot, openAI)
			view := candidates.view
			view.Decision = gatewayRoutingDecision(GatewayPreflightRequest{Model: model, Protocol: protocol}, candidates, false)
			entry.Protocols[protocol] = view
		}
		result.Models = append(result.Models, entry)
	}
	return result
}

// PreflightGatewayRequest evaluates evidence for a request shape; a positive
// answer is not admission, a reservation, or an upstream success guarantee.
func (s *GatewayService) PreflightGatewayRequest(ctx context.Context, group *Group, request GatewayPreflightRequest, openAI *OpenAIGatewayService) (GatewayPreflightResponse, error) {
	result := GatewayPreflightResponse{SchemaVersion: 2, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Advisory: true, Model: request.Model, Protocol: request.Protocol}
	if err := request.Validate(); err != nil {
		return result, err
	}
	if group != nil && !group.ModelAllowlist.Allows(request.Model) {
		result.Catalog = GatewayEffectiveCatalog{Discovery: "unknown"}
		result.Routing = GatewayEffectiveState{"restricted", "MODEL_NOT_ALLOWED"}
		result.Availability = GatewayEffectiveState{State: "not_applicable"}
		result.Support = GatewayEffectiveState{"unsupported", "MODEL_NOT_ALLOWED"}
		result.Decision = gatewayRoutingDecision(request, gatewayEffectiveCandidates{view: GatewayEffectiveProtocol{Routing: result.Routing, Availability: result.Availability}}, true)
		return result, nil
	}
	snapshot := s.loadGatewayEffectiveSnapshot(ctx, group)
	result.GeneratedAt = snapshot.observedAt.UTC().Format(time.RFC3339Nano)
	published := slices.Contains(s.gatewayEffectiveModelIDs(ctx, group, snapshot), request.Model)
	result.Catalog = s.gatewayEffectiveCatalog(ctx, group, request.Model, published, snapshot)
	candidates := s.gatewayEffectiveCandidates(ctx, group, request.Model, request.Protocol, snapshot, openAI)
	result.Decision = gatewayRoutingDecision(request, candidates, true)
	result.Routing = candidates.view.Routing
	result.Availability = candidates.view.Availability
	result.Support = gatewayEvaluateRequestSets(request, candidates.configured)
	if result.Routing.State != "configured" {
		result.Support = GatewayEffectiveState{"unknown", "UNKNOWN"}
		if result.Routing.State == "restricted" || result.Routing.State == "not_configured" {
			result.Support = GatewayEffectiveState{"unsupported", result.Routing.Reason}
		}
		return result, nil
	}
	if result.Support.State == "unsupported" {
		result.Availability = GatewayEffectiveState{State: "not_applicable"}
	} else if result.Availability.State == "available" {
		currentSupport := gatewayEvaluateRequestSets(request, candidates.current)
		switch currentSupport.State {
		case "unsupported":
			result.Availability = GatewayEffectiveState{"temporarily_unavailable", "TEMPORARILY_UNAVAILABLE"}
		case "unknown", "conditional":
			result.Availability = GatewayEffectiveState{"unknown", "UNKNOWN"}
		}
	}
	return result, nil
}

func (s *GatewayService) gatewayEffectiveModelIDs(ctx context.Context, group *Group, snapshot gatewayCapabilitySnapshot) []string {
	// A stale scheduler snapshot cannot republish removed account policy.
	ids := gatewayCapabilityVisibleModelIDsWithSource(group, nil, false, snapshot.configured, snapshot.configuredKnown, nil, false, DefaultGatewayCapabilityFallbacks(), func(accounts []Account, platform string) []string {
		return availableModelIDsFromAccountsWithManifestIDs(accounts, platform, func(account *Account) []string { return snapshot.manifestIDs[account.ID] })
	})
	if group != nil && group.Platform == PlatformComposite && snapshot.routesKnown {
		for _, route := range snapshot.routes {
			if !route.Enabled || normalizeCompositeRouteMatchType(route.MatchType) != CompositeRouteMatchExact {
				continue
			}
			endpoint := normalizeCompositeRouteEndpoint(route.Endpoint)
			if endpoint == CompositeRouteEndpointAny || slices.Contains(gatewayEffectiveProtocols, endpoint) {
				ids = mergeGatewayCapabilityModelIDs(ids, []string{route.PublicModel})
			}
		}
		if group.CustomModelsListEnabled() {
			ids = FilterModelsByCustomList(ids, nil, group.ModelsListConfig.Models)
		}
	}
	if group != nil && group.Platform == PlatformComposite {
		backed := make([]string, 0, len(ids))
		for _, model := range ids {
			for _, protocol := range gatewayEffectiveProtocols {
				route := gatewayCapabilityRouteForEndpoint(group, model, protocol, snapshot.routes, snapshot.routesKnown, snapshot.configured, s != nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple)
				if len(s.gatewayCapabilitySupportingAccounts(WithCompositeRouteDecision(ctx, route.decision), snapshot.configured, route, false)) > 0 {
					backed = append(backed, model)
					break
				}
			}
		}
		ids = backed
	}

	if group != nil {
		ids = group.ModelAllowlist.FilterForListing(ids)
	}
	sort.Strings(ids)
	return ids
}

func (s *GatewayService) gatewayEffectiveCatalog(ctx context.Context, group *Group, model string, published bool, snapshot gatewayCapabilitySnapshot) GatewayEffectiveCatalog {
	result := GatewayEffectiveCatalog{Published: published, Discovery: "unknown"}
	for _, protocol := range gatewayEffectiveProtocols {
		route := gatewayCapabilityRouteForEndpoint(group, model, protocol, snapshot.routes, snapshot.routesKnown, snapshot.configured, s != nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple)
		accounts := s.gatewayCapabilitySupportingAccounts(WithCompositeRouteDecision(ctx, route.decision), snapshot.configured, route, false)
		for i := range accounts {
			account := &accounts[i]
			mapped := strings.TrimPrefix(strings.TrimSpace(account.GetMappedModel(route.upstreamModel)), "models/")
			if snapshot.observedModels[account.ID][mapped] {
				result.Discovery = "observed"
				return result
			}
		}
	}
	return result
}

func (s *GatewayService) gatewayEffectiveCandidates(ctx context.Context, group *Group, model, protocol string, snapshot gatewayCapabilitySnapshot, openAI *OpenAIGatewayService) gatewayEffectiveCandidates {
	result := gatewayEffectiveCandidates{view: GatewayEffectiveProtocol{
		Routing: GatewayEffectiveState{"unknown", "UNKNOWN"}, Availability: GatewayEffectiveState{"unknown", "UNKNOWN"},
		Capabilities: gatewayIntersectFeatures(nil),
	}}
	if !snapshot.configuredKnown || !snapshot.routesKnown {
		return result
	}
	preferDetected := s != nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple
	route := gatewayCapabilityRouteForEndpoint(group, model, protocol, snapshot.routes, snapshot.routesKnown, snapshot.configured, preferDetected)
	if !route.known {
		return result
	}
	if route.decision.Matched {
		ctx = WithCompositeRouteDecision(ctx, route.decision)
	}
	if protocol == CompositeRouteEndpointMessages && route.targetPlatform == PlatformOpenAI && !GroupAllowsMessagesDispatch(group, route.targetPlatform) {
		result.view.Routing = GatewayEffectiveState{"restricted", "OPERATOR_RESTRICTED"}
		result.view.Availability = GatewayEffectiveState{State: "not_applicable"}
		return result
	}
	if group != nil && group.ClaudeCodeOnly && (route.targetPlatform == PlatformAnthropic || route.targetPlatform == PlatformGemini || route.targetPlatform == PlatformAntigravity) {
		if protocol == CompositeRouteEndpointMessages {
			// Client identity and fallback-group admission require the real request.
			result.reason = "REQUEST_DEPENDENT_POLICY_UNKNOWN"
			return result
		}
		result.view.Routing = GatewayEffectiveState{"restricted", "OPERATOR_RESTRICTED"}
		result.view.Availability = GatewayEffectiveState{State: "not_applicable"}
		return result
	}
	if group != nil && group.ProfitControlEnabled {
		// Token-cost eligibility requires request pricing and input size.
		result.reason = "REQUEST_DEPENDENT_POLICY_UNKNOWN"
		return result
	}
	var groupID *int64
	if group != nil && group.ID > 0 {
		groupID = &group.ID
	}
	channelKnown := true
	if s != nil && s.channelService != nil && groupID != nil {
		_, err := s.channelService.GetChannelForGroup(ctx, *groupID)
		channelKnown = err == nil
	}
	if !channelKnown {
		return result
	}
	// Match live ordering: Composite rewrite, then channel mapping, then the
	// per-account mapping. Retain the public name in the Composite context.
	selectionModel := route.upstreamModel
	if s != nil {
		mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, groupID, selectionModel)
		if protocol == CompositeRouteEndpointMessages && (route.targetPlatform == PlatformOpenAI || route.targetPlatform == PlatformGrok || IsCNProvider(route.targetPlatform)) {
			// Messages can select a dispatch model and later forward a different
			// channel model, with request-time fallback. A passive snapshot cannot
			// promise a single effective shape across those branches.
			if mapping.Mapped || group != nil && group.ResolveMessagesDispatchModel(selectionModel) != "" &&
				(group.Platform != PlatformComposite || route.targetPlatform == PlatformOpenAI) {
				result.reason = "REQUEST_DEPENDENT_POLICY_UNKNOWN"
				return result
			}
			selectionModel = NormalizeOpenAICompatRequestedModel(selectionModel)
		} else if mapping.Mapped && strings.TrimSpace(mapping.MappedModel) != "" {
			selectionModel = mapping.MappedModel
		}
	}

	if s != nil && s.checkChannelPricingRestriction(ctx, groupID, selectionModel) {
		result.view.Routing = GatewayEffectiveState{"restricted", "OPERATOR_RESTRICTED"}
		result.view.Availability = GatewayEffectiveState{State: "not_applicable"}
		return result
	}
	route.upstreamModel = selectionModel
	configured := s.gatewayCapabilitySupportingAccounts(ctx, snapshot.configured, route, false)
	pool := snapshot.currentByPlatform[route.targetPlatform]
	currentByID := make(map[int64]Account, len(pool.accounts))
	for _, account := range pool.accounts {
		currentByID[account.ID] = account
	}
	upstreamRestriction := s != nil && s.needsUpstreamChannelRestrictionCheck(ctx, groupID)
	policyBlocked := false
	protocolBlocked := false
	currentUnknown := false
	for i := range configured {
		account := &configured[i]
		if protocol == CompositeRouteEndpointChatCompletions && !GatewayChatAccountCompatible(route.targetPlatform, account) {
			protocolBlocked = true
			continue
		}
		if group != nil && group.RequirePrivacySet && !account.IsPrivacySet() {
			policyBlocked = true
			continue
		}
		upstreamModel := resolveAccountUpstreamModel(account, selectionModel)
		if account.IsOpenAICompatible() {
			upstreamModel = resolveOpenAIAccountUpstreamModelForRequest(account, selectionModel, false)
		}
		if upstreamRestriction && s.channelService.IsModelRestricted(ctx, *groupID, upstreamModel) {
			policyBlocked = true
			continue
		}
		features := gatewayAccountFeaturesWithMetadata(account, upstreamModel, protocol, snapshot.metadata[account.ID])
		if group != nil && (group.MaxReasoningEffort != "" || len(group.ReasoningEffortMappings) > 0) {
			features.ReasoningEfforts = nil
			features.Features["reasoning"] = "unknown"
		}
		if features.Features["protocol"] == "unsupported" {
			protocolBlocked = true
			continue
		}
		result.configured = append(result.configured, features)
		current, ok := currentByID[account.ID]
		observation := GatewayEffectiveState{"unknown", "AVAILABILITY_UNKNOWN"}
		if pool.known {
			observation = gatewayCandidateAvailability(ctx, account, current, ok, selectionModel, openAI, snapshot.observedAt)
		}
		result.observations = append(result.observations, observation)
		switch observation.State {
		case "unknown":
			currentUnknown = true
			continue
		case "temporarily_unavailable":
			continue
		}

		result.current = append(result.current, features)
	}
	result.view.Capabilities = gatewayIntersectFeatures(result.configured)
	if len(result.configured) == 0 {
		result.view.Routing = GatewayEffectiveState{"not_configured", "NO_CONFIGURED_ROUTE"}
		if protocolBlocked {
			result.reason = "PROTOCOL_UNSUPPORTED"
		}
		if policyBlocked {
			result.reason = "OPERATOR_RESTRICTED"
			result.view.Routing = GatewayEffectiveState{"restricted", "OPERATOR_RESTRICTED"}
		}
		result.view.Availability = GatewayEffectiveState{State: "not_applicable"}
		return result
	}
	result.view.Routing = GatewayEffectiveState{State: "configured"}
	if pool.known {
		switch {
		case len(result.current) > 0:
			result.view.Availability = GatewayEffectiveState{State: "available"}
		case currentUnknown:
			// Keep unknown when all remaining paths need unavailable evidence.
		default:
			result.view.Availability = GatewayEffectiveState{"temporarily_unavailable", "TEMPORARILY_UNAVAILABLE"}
		}
	}
	return result
}

func gatewayEvaluateRequestSets(request GatewayPreflightRequest, sets []GatewayFeatureSet) GatewayEffectiveState {
	if len(sets) == 0 {
		return GatewayEffectiveState{"unknown", "UNKNOWN"}
	}
	states := make([]GatewayEffectiveState, 0, len(sets))
	for _, set := range sets {
		states = append(states, gatewayEvaluateRequest(request, set))
	}
	supported, unsupported := 0, 0
	reason := "CAPABILITY_UNSUPPORTED"
	for _, state := range states {
		switch state.State {
		case "supported":
			supported++
		case "unsupported":
			unsupported++
			if state.Reason == "CAPABILITY_COMBINATION_UNSUPPORTED" {
				reason = state.Reason
			}
		}
	}
	switch {
	case supported == len(states):
		return GatewayEffectiveState{State: "supported"}
	case unsupported == len(states):
		return GatewayEffectiveState{"unsupported", reason}
	case supported > 0:
		return GatewayEffectiveState{"conditional", "ROUTE_DEPENDENT"}
	default:
		return GatewayEffectiveState{"unknown", "UNKNOWN"}
	}
}

func gatewayEvaluateRequest(request GatewayPreflightRequest, set GatewayFeatureSet) GatewayEffectiveState {
	features := []string{"protocol"}
	for _, modality := range request.InputModalities {
		features = append(features, modality+"_input")
	}
	features = append(features, request.Tools...)
	if request.ReasoningEffort != "" && request.ReasoningEffort != "none" {
		features = append(features, "reasoning")
	}
	if request.Streaming {
		features = append(features, "streaming")
	}
	if request.ServiceTier != "" {
		features = append(features, "service_tier."+request.ServiceTier)
	}
	requestedFeatures := make(map[string]bool, len(features))
	for _, feature := range features {
		requestedFeatures[feature] = true
	}
	for _, constraint := range set.Constraints {
		if gatewayFeatureConstraintMatches(constraint, requestedFeatures) {
			return GatewayEffectiveState{"unsupported", "CAPABILITY_COMBINATION_UNSUPPORTED"}
		}
	}

	unknown := false
	for _, feature := range features {
		switch set.Features[feature] {
		case "supported":
		case "unsupported":
			return GatewayEffectiveState{"unsupported", "CAPABILITY_UNSUPPORTED"}
		default:
			unknown = true
		}
	}
	if request.ReasoningEffort != "" {
		if len(set.ReasoningEfforts) == 0 {
			unknown = true
		} else if !slices.Contains(set.ReasoningEfforts, request.ReasoningEffort) {
			return GatewayEffectiveState{"unsupported", "CAPABILITY_UNSUPPORTED"}
		}
	}
	if unknown {
		return GatewayEffectiveState{"unknown", "UNKNOWN"}
	}
	return GatewayEffectiveState{State: "supported"}
}
