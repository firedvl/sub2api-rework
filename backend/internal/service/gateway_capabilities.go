package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	GatewayAvailabilityAvailable   = "available"
	GatewayAvailabilityDegraded    = "degraded"
	GatewayAvailabilityUnavailable = "unavailable"
	GatewayAvailabilityUnknown     = "unknown"

	GatewayRouteDirect    = "direct"
	GatewayRouteComposite = "composite"
	GatewayRouteAlias     = "alias"
	GatewayRouteUnknown   = "unknown"

	GatewayCapacityKnown   = "known"
	GatewayCapacityPartial = "partial"
	GatewayCapacityUnknown = "unknown"
)

var gatewayCapabilityPlatforms = []string{
	PlatformAnthropic,
	PlatformGemini,
	PlatformOpenAI,
	PlatformAntigravity,
	PlatformGrok,
	PlatformKimi,
	PlatformZhipu,
	PlatformDeepseek,
}

// GatewayCapabilityModel is an explicit public DTO. Account and route records
// never cross the gateway discovery boundary.
type GatewayCapabilityModel struct {
	ID           string                    `json:"id"`
	DisplayName  string                    `json:"display_name"`
	Availability string                    `json:"availability"`
	Capabilities *GatewayModelCapabilities `json:"capabilities,omitempty"`
	Routing      GatewayCapabilityRouting  `json:"routing"`
	Capacity     GatewayCapabilityCapacity `json:"capacity"`
}

type GatewayModelCapabilities struct {
	Reasoning   *bool `json:"reasoning,omitempty"`
	ImageOutput *bool `json:"image_output,omitempty"`
}

type GatewayCapabilityRouting struct {
	Routable       *bool  `json:"routable,omitempty"`
	Type           string `json:"type"`
	CandidatePaths *int   `json:"candidate_paths,omitempty"`
}

type GatewayCapabilityCapacity struct {
	Status                   string                  `json:"status"`
	Windows                  []GatewayCapacityWindow `json:"windows,omitempty"`
	LimitingRemainingPercent *float64                `json:"limiting_remaining_percent,omitempty"`
	NextResetAt              string                  `json:"next_reset_at,omitempty"`
}

type GatewayCapacityWindow struct {
	Window           string  `json:"window"`
	RemainingPercent float64 `json:"remaining_percent"`
	ResetAt          string  `json:"reset_at,omitempty"`
}

type gatewayCapabilityRoute struct {
	targetPlatform string
	upstreamModel  string
	routeType      string
	known          bool
	decision       CompositeRouteDecision
}

type gatewayCapabilityAccountPool struct {
	accounts []Account
	known    bool
}

// BuildGatewayCapabilityModels builds an API-key-scoped model snapshot from
// passive scheduler and repository state. It never fills a missing scheduler
// snapshot as a side effect of discovery.
func (s *GatewayService) BuildGatewayCapabilityModels(
	ctx context.Context,
	group *Group,
	fallbacks map[string][]string,
) []GatewayCapabilityModel {
	var current []Account
	currentKnown := false
	var currentByPlatform map[string]gatewayCapabilityAccountPool
	if s != nil && s.schedulerSnapshot != nil {
		currentByPlatform = make(map[string]gatewayCapabilityAccountPool, len(gatewayCapabilityPlatforms))
		platforms := gatewayCapabilityPlatforms
		if group == nil || group.Platform != PlatformComposite {
			platform := PlatformAnthropic
			if group != nil && strings.TrimSpace(group.Platform) != "" {
				platform = group.Platform
			}
			platforms = []string{platform}
		}
		var groupID *int64
		if group != nil && group.ID > 0 {
			id := group.ID
			groupID = &id
		}
		for _, platform := range platforms {
			accounts, _, hit, err := s.schedulerSnapshot.PeekSchedulableAccounts(ctx, groupID, platform, false)
			known := err == nil && hit
			if known {
				accounts = s.filterGatewayCapabilityCurrentAccounts(ctx, accounts)
				current = append(current, accounts...)
				currentKnown = true
			}
			currentByPlatform[platform] = gatewayCapabilityAccountPool{accounts: accounts, known: known}
		}
	}
	configured, configuredKnown := s.gatewayCapabilityConfiguredAccounts(ctx, group)
	routes, routesKnown := s.gatewayCapabilityCompositeRoutes(ctx, group)
	modelIDs := gatewayCapabilityVisibleModelIDs(group, current, currentKnown, configured, configuredKnown, routes, routesKnown, fallbacks)

	models := make([]GatewayCapabilityModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		route := gatewayCapabilityRouteForModel(group, modelID, routes, routesKnown)
		modelCurrent, modelCurrentKnown := current, currentKnown
		if currentByPlatform != nil {
			pool := currentByPlatform[route.targetPlatform]
			modelCurrent, modelCurrentKnown = pool.accounts, pool.known
		}
		modelCtx := ctx
		if route.decision.Matched {
			modelCtx = WithCompositeRouteDecision(modelCtx, route.decision)
		}
		configuredPaths := s.gatewayCapabilitySupportingAccounts(modelCtx, configured, route, false)
		currentPaths := s.gatewayCapabilitySupportingAccounts(modelCtx, modelCurrent, route, true)

		availability := GatewayAvailabilityUnknown
		if route.known && modelCurrentKnown {
			switch {
			case len(currentPaths) == 0:
				availability = GatewayAvailabilityUnavailable
			case configuredKnown && len(configuredPaths) > len(currentPaths):
				availability = GatewayAvailabilityDegraded
			default:
				availability = GatewayAvailabilityAvailable
			}
		}

		routeType := route.routeType
		if routeType == GatewayRouteDirect && gatewayCapabilityAllPathsAlias(modelID, configuredPaths, currentPaths) {
			routeType = GatewayRouteAlias
		}
		routing := GatewayCapabilityRouting{Type: routeType}
		if route.known && modelCurrentKnown {
			candidatePaths := len(currentPaths)
			routable := candidatePaths > 0
			routing.Routable = &routable
			routing.CandidatePaths = &candidatePaths
		}

		capacity := GatewayCapabilityCapacity{Status: GatewayCapacityUnknown}
		if route.known && modelCurrentKnown && len(currentPaths) > 0 {
			capacity = gatewayCapabilityCapacity(currentPaths, route.upstreamModel, time.Now())
		}
		models = append(models, GatewayCapabilityModel{
			ID:           modelID,
			DisplayName:  modelID,
			Availability: availability,
			Routing:      routing,
			Capacity:     capacity,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func (s *GatewayService) filterGatewayCapabilityCurrentAccounts(ctx context.Context, accounts []Account) []Account {
	if len(accounts) == 0 {
		return accounts
	}
	if s.rateLimitService != nil && s.rateLimitService.settingService != nil {
		thresholds := s.rateLimitService.settingService.GetAccountSchedulingThresholds(ctx)
		now := time.Now().UTC()
		filtered := make([]Account, 0, len(accounts))
		for i := range accounts {
			if EvaluateAccountSchedulingThreshold(&accounts[i], thresholds, now).ShouldPause {
				continue
			}
			filtered = append(filtered, accounts[i])
		}
		accounts = filtered
	}
	return filterGrokFreeQuotaAccountsCore(ctx, s.cfg, s.usageLogRepo, &gatewayGrokFreeQuotaGateCache, accounts, false)
}

func (s *GatewayService) gatewayCapabilityConfiguredAccounts(ctx context.Context, group *Group) ([]Account, bool) {
	if s == nil || s.accountRepo == nil {
		return nil, false
	}
	var groupID *int64
	includeGrouped := s.cfg != nil && s.cfg.RunMode == config.RunModeSimple
	if group != nil && group.ID > 0 {
		id := group.ID
		groupID = &id
		includeGrouped = false
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, groupID, gatewayCapabilityPlatforms, includeGrouped)
	return accounts, err == nil
}

func (s *GatewayService) gatewayCapabilityCompositeRoutes(ctx context.Context, group *Group) ([]CompositeModelRoute, bool) {
	if group == nil || group.Platform != PlatformComposite {
		return nil, true
	}
	if s == nil || s.compositeResolver == nil || s.compositeResolver.repo == nil {
		return nil, false
	}
	routes, err := s.compositeResolver.repo.ListByGroup(ctx, group.ID, false)
	return routes, err == nil
}

func gatewayCapabilityVisibleModelIDs(
	group *Group,
	current []Account,
	currentKnown bool,
	configured []Account,
	configuredKnown bool,
	routes []CompositeModelRoute,
	routesKnown bool,
	fallbacks map[string][]string,
) []string {
	platform := PlatformAnthropic
	if group != nil && strings.TrimSpace(group.Platform) != "" {
		platform = group.Platform
	}

	if platform != PlatformComposite {
		available := make([]string, 0)
		if configuredKnown {
			available = mergeGatewayCapabilityModelIDs(available, availableModelIDsFromAccounts(configured, platform))
		}
		if currentKnown {
			available = mergeGatewayCapabilityModelIDs(available, availableModelIDsFromAccounts(current, platform))
		}
		fallback := cloneStringSlice(fallbacks[platform])
		if group != nil && group.CustomModelsListEnabled() {
			if platform == PlatformAnthropic && len(available) > 0 {
				available = mergeGatewayCapabilityModelIDs(available, fallback)
			}
			return FilterModelsByCustomList(available, fallback, group.ModelsListConfig.Models)
		}
		if len(available) > 0 {
			return available
		}
		return fallback
	}

	available := make([]string, 0)
	for _, concrete := range gatewayCapabilityPlatforms {
		platformModels := make([]string, 0)
		hasPlatform := false
		if configuredKnown {
			platformModels = mergeGatewayCapabilityModelIDs(platformModels, availableModelIDsFromAccounts(configured, concrete))
			hasPlatform = gatewayCapabilityHasPlatform(configured, concrete)
		}
		if currentKnown {
			platformModels = mergeGatewayCapabilityModelIDs(platformModels, availableModelIDsFromAccounts(current, concrete))
			hasPlatform = hasPlatform || gatewayCapabilityHasPlatform(current, concrete)
		}
		if len(platformModels) == 0 && hasPlatform && !IsCNProvider(concrete) {
			platformModels = fallbacks[concrete]
		}
		available = mergeGatewayCapabilityModelIDs(available, platformModels)
	}
	if routesKnown {
		available = mergeGatewayCapabilityModelIDs(available, gatewayCapabilityExactRouteModelIDs(routes))
	}
	fallback := cloneStringSlice(fallbacks[PlatformComposite])
	if group != nil && group.CustomModelsListEnabled() {
		return FilterModelsByCustomList(available, fallback, group.ModelsListConfig.Models)
	}
	if len(available) > 0 {
		return available
	}
	return fallback
}

func gatewayCapabilityExactRouteModelIDs(routes []CompositeModelRoute) []string {
	models := make([]string, 0, len(routes))
	for _, route := range routes {
		if normalizeCompositeRouteMatchType(route.MatchType) != CompositeRouteMatchExact {
			continue
		}
		endpoint := normalizeCompositeRouteEndpoint(route.Endpoint)
		if endpoint != CompositeRouteEndpointResponses && endpoint != CompositeRouteEndpointAny {
			continue
		}
		models = append(models, route.PublicModel)
	}
	return models
}

func gatewayCapabilityHasPlatform(accounts []Account, platform string) bool {
	for i := range accounts {
		if accounts[i].Platform == platform {
			return true
		}
	}
	return false
}

func mergeGatewayCapabilityModelIDs(first, second []string) []string {
	seen := make(map[string]struct{}, len(first)+len(second))
	result := make([]string, 0, len(first)+len(second))
	for _, models := range [][]string{first, second} {
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			result = append(result, model)
		}
	}
	return result
}

func gatewayCapabilityRouteForModel(group *Group, model string, routes []CompositeModelRoute, routesKnown bool) gatewayCapabilityRoute {
	platform := PlatformAnthropic
	if group != nil && strings.TrimSpace(group.Platform) != "" {
		platform = group.Platform
	}
	if platform != PlatformComposite {
		return gatewayCapabilityRoute{targetPlatform: platform, upstreamModel: model, routeType: GatewayRouteDirect, known: platform != ""}
	}
	if !routesKnown {
		return gatewayCapabilityRoute{upstreamModel: model, routeType: GatewayRouteUnknown}
	}
	if explicit, ok := matchCompositeRoute(routes, model, CompositeRouteEndpointResponses, true); ok {
		upstreamModel := strings.TrimSpace(explicit.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = model
		}
		decision := CompositeRouteDecision{
			Matched:        true,
			Source:         CompositeRouteSourceExplicit,
			GroupID:        group.ID,
			PublicModel:    model,
			TargetPlatform: explicit.TargetPlatform,
			UpstreamModel:  upstreamModel,
			Endpoint:       CompositeRouteEndpointResponses,
		}
		return gatewayCapabilityRoute{targetPlatform: explicit.TargetPlatform, upstreamModel: upstreamModel, routeType: GatewayRouteComposite, known: isConcreteRequestPlatform(explicit.TargetPlatform), decision: decision}
	}
	if detected, ok := DetectModelPlatform(model); ok {
		decision := CompositeRouteDecision{
			Matched:        true,
			Source:         CompositeRouteSourceDetector,
			GroupID:        group.ID,
			PublicModel:    model,
			TargetPlatform: detected,
			UpstreamModel:  model,
			Endpoint:       CompositeRouteEndpointResponses,
		}
		return gatewayCapabilityRoute{targetPlatform: detected, upstreamModel: model, routeType: GatewayRouteComposite, known: true, decision: decision}
	}
	return gatewayCapabilityRoute{upstreamModel: model, routeType: GatewayRouteUnknown}
}

func (s *GatewayService) gatewayCapabilitySupportingAccounts(ctx context.Context, accounts []Account, route gatewayCapabilityRoute, current bool) []Account {
	if s == nil || !route.known {
		return nil
	}
	useMixed := route.targetPlatform == PlatformAnthropic || route.targetPlatform == PlatformGemini
	result := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !s.isAccountAllowedForPlatform(account, route.targetPlatform, useMixed) ||
			!s.isModelSupportedByAccountWithContext(ctx, account, route.upstreamModel) {
			continue
		}
		if current && !account.IsSchedulableForModelWithContext(ctx, route.upstreamModel) {
			continue
		}
		result = append(result, *account)
	}
	return result
}

func gatewayCapabilityAllPathsAlias(model string, pathSets ...[]Account) bool {
	paths := make([]Account, 0)
	for _, set := range pathSets {
		if len(set) > 0 {
			paths = set
			break
		}
	}
	if len(paths) == 0 {
		return false
	}
	for i := range paths {
		mapped, matched := paths[i].ResolveMappedModel(model)
		if !matched || strings.TrimSpace(mapped) == strings.TrimSpace(model) {
			return false
		}
	}
	return true
}

type gatewayAccountCapacity struct {
	windows  []GatewayCapacityWindow
	limiting float64
	resetAt  string
}

func gatewayCapabilityCapacity(accounts []Account, model string, now time.Time) GatewayCapabilityCapacity {
	knownAccounts := 0
	var best *gatewayAccountCapacity
	for i := range accounts {
		candidate := gatewayCapacityForAccount(&accounts[i], model, now)
		if candidate == nil {
			continue
		}
		knownAccounts++
		if best == nil || candidate.limiting > best.limiting {
			best = candidate
		}
	}
	if best == nil {
		return GatewayCapabilityCapacity{Status: GatewayCapacityUnknown}
	}
	status := GatewayCapacityKnown
	if knownAccounts < len(accounts) {
		status = GatewayCapacityPartial
	}
	limiting := best.limiting
	return GatewayCapabilityCapacity{
		Status:                   status,
		Windows:                  best.windows,
		LimitingRemainingPercent: &limiting,
		NextResetAt:              best.resetAt,
	}
}

func gatewayCapacityForAccount(account *Account, model string, now time.Time) *gatewayAccountCapacity {
	if account == nil {
		return nil
	}
	windows := make(map[string]GatewayCapacityWindow)
	add := func(window GatewayCapacityWindow) {
		window.Window = strings.TrimSpace(window.Window)
		if window.Window == "" || math.IsNaN(window.RemainingPercent) || math.IsInf(window.RemainingPercent, 0) {
			return
		}
		window.RemainingPercent = clampGatewayPercent(window.RemainingPercent)
		if previous, ok := windows[window.Window]; !ok || window.RemainingPercent < previous.RemainingPercent {
			windows[window.Window] = window
		}
	}

	if account.IsAPIKeyOrBedrock() {
		addLocalQuotaWindows(account, now, add)
	}
	switch account.Platform {
	case PlatformOpenAI:
		for _, window := range []string{"5h", "7d"} {
			if progress := buildCodexUsageProgressFromExtra(account.Extra, window, now); progress != nil {
				add(gatewayWindowFromUsage(window, progress, now))
			}
		}
	case PlatformAnthropic:
		addAnthropicCapacityWindows(account, model, now, add)
	case PlatformGrok:
		if snapshot, err := grokQuotaSnapshotFromExtra(account.Extra); err == nil && snapshot != nil {
			addGrokCapacityWindow("requests", snapshot.Requests, now, add)
			addGrokCapacityWindow("tokens", snapshot.Tokens, now, add)
		}
	case PlatformKimi, PlatformZhipu, PlatformDeepseek:
		addCNCapacityWindows(account, now, add)
	}

	if len(windows) == 0 {
		return nil
	}
	ordered := make([]GatewayCapacityWindow, 0, len(windows))
	for _, window := range windows {
		ordered = append(ordered, window)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Window < ordered[j].Window })
	limiting := ordered[0]
	for _, window := range ordered[1:] {
		if window.RemainingPercent < limiting.RemainingPercent {
			limiting = window
		}
	}
	return &gatewayAccountCapacity{windows: ordered, limiting: limiting.RemainingPercent, resetAt: limiting.ResetAt}
}

func addLocalQuotaWindows(account *Account, now time.Time, add func(GatewayCapacityWindow)) {
	if limit := account.GetQuotaLimit(); limit > 0 {
		add(GatewayCapacityWindow{Window: "total", RemainingPercent: remainingGatewayPercent(account.GetQuotaUsed(), limit)})
	}
	if limit := account.GetQuotaDailyLimit(); limit > 0 {
		used := account.GetQuotaDailyUsed()
		resetAt := gatewayLocalQuotaReset(account, "daily", now)
		if account.IsDailyQuotaPeriodExpired() {
			used = 0
			resetAt = time.Time{}
		}
		add(GatewayCapacityWindow{Window: "daily", RemainingPercent: remainingGatewayPercent(used, limit), ResetAt: gatewayTime(resetAt)})
	}
	if limit := account.GetQuotaWeeklyLimit(); limit > 0 {
		used := account.GetQuotaWeeklyUsed()
		resetAt := gatewayLocalQuotaReset(account, "weekly", now)
		if account.IsWeeklyQuotaPeriodExpired() {
			used = 0
			resetAt = time.Time{}
		}
		add(GatewayCapacityWindow{Window: "weekly", RemainingPercent: remainingGatewayPercent(used, limit), ResetAt: gatewayTime(resetAt)})
	}
}

func gatewayLocalQuotaReset(account *Account, window string, now time.Time) time.Time {
	if window == "daily" {
		if account.GetQuotaDailyResetMode() == "fixed" {
			if reset := account.getExtraTime("quota_daily_reset_at"); reset.After(now) {
				return reset
			}
			location, err := time.LoadLocation(account.GetQuotaResetTimezone())
			if err != nil {
				location = time.UTC
			}
			return nextFixedDailyReset(account.GetQuotaDailyResetHour(), location, now)
		}
		return account.getExtraTime("quota_daily_start").Add(24 * time.Hour)
	}
	if account.GetQuotaWeeklyResetMode() == "fixed" {
		if reset := account.getExtraTime("quota_weekly_reset_at"); reset.After(now) {
			return reset
		}
		location, err := time.LoadLocation(account.GetQuotaResetTimezone())
		if err != nil {
			location = time.UTC
		}
		return nextFixedWeeklyReset(account.GetQuotaWeeklyResetDay(), account.GetQuotaWeeklyResetHour(), location, now)
	}
	return account.getExtraTime("quota_weekly_start").Add(7 * 24 * time.Hour)
}

func addAnthropicCapacityWindows(account *Account, model string, now time.Time, add func(GatewayCapacityWindow)) {
	if raw, ok := account.Extra["session_window_utilization"]; ok || account.SessionWindowEnd != nil {
		used := parseExtraFloat64(raw) * 100
		resetAt := time.Time{}
		if account.SessionWindowEnd != nil {
			resetAt = *account.SessionWindowEnd
			if !now.Before(resetAt) {
				used = 0
				resetAt = time.Time{}
			}
		}
		add(GatewayCapacityWindow{Window: "5h", RemainingPercent: remainingGatewayPercent(used, 100), ResetAt: gatewayTime(resetAt)})
	}
	if progress := buildPassiveUsageWindow(account.Extra, "passive_usage_7d_utilization", "passive_usage_7d_reset"); progress != nil {
		add(gatewayWindowFromUsage("7d", progress, now))
	}
	if isAnthropicFableModel(model) {
		if progress := buildPassiveUsageWindow(account.Extra, "passive_usage_7d_oi_utilization", "passive_usage_7d_oi_reset"); progress != nil {
			add(gatewayWindowFromUsage("7d_fable", progress, now))
		}
	}
}

func addGrokCapacityWindow(name string, window *xai.QuotaWindow, now time.Time, add func(GatewayCapacityWindow)) {
	if window == nil || window.Limit == nil || window.Remaining == nil || *window.Limit <= 0 {
		return
	}
	remaining := (float64(*window.Remaining) / float64(*window.Limit)) * 100
	resetAt := time.Time{}
	if window.ResetUnix != nil && *window.ResetUnix > 0 {
		resetAt = time.Unix(*window.ResetUnix, 0)
		if !now.Before(resetAt) {
			remaining = 100
			resetAt = time.Time{}
		}
	}
	add(GatewayCapacityWindow{Window: name, RemainingPercent: remaining, ResetAt: gatewayTime(resetAt)})
}

func addCNCapacityWindows(account *Account, now time.Time, add func(GatewayCapacityWindow)) {
	for _, item := range []struct {
		name     string
		usedKey  string
		resetKey string
	}{
		{name: "5h", usedKey: cnExtraKey(account.Platform, cnExtraSuffix5hUsed), resetKey: cnExtraKey(account.Platform, cnExtraSuffix5hReset)},
		{name: "weekly", usedKey: cnExtraKey(account.Platform, cnExtraSuffixWeeklyUsed), resetKey: cnExtraKey(account.Platform, cnExtraSuffixWeeklyReset)},
	} {
		usedRaw, ok := account.Extra[item.usedKey]
		if !ok {
			continue
		}
		used := parseExtraFloat64(usedRaw)
		resetAt := time.Time{}
		if raw, ok := account.Extra[item.resetKey]; ok {
			resetAt, _ = parseTime(fmt.Sprint(raw))
			if !resetAt.IsZero() && !now.Before(resetAt) {
				used = 0
				resetAt = time.Time{}
			}
		}
		add(GatewayCapacityWindow{Window: item.name, RemainingPercent: remainingGatewayPercent(used, 100), ResetAt: gatewayTime(resetAt)})
	}
}

func gatewayWindowFromUsage(name string, progress *UsageProgress, now time.Time) GatewayCapacityWindow {
	if progress.ResetsAt != nil && !now.Before(*progress.ResetsAt) {
		return GatewayCapacityWindow{Window: name, RemainingPercent: 100}
	}
	window := GatewayCapacityWindow{Window: name, RemainingPercent: remainingGatewayPercent(progress.Utilization, 100)}
	if progress.ResetsAt != nil {
		window.ResetAt = gatewayTime(*progress.ResetsAt)
	}
	return window
}

func remainingGatewayPercent(used, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	return clampGatewayPercent(((limit - used) / limit) * 100)
}

func clampGatewayPercent(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}

func gatewayTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
