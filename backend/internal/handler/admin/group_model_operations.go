package admin

import (
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const modelOperationsWindow = 7 * 24 * time.Hour

type modelOperationsResponse struct {
	GeneratedAt string                   `json:"generated_at"`
	Window      modelOperationsWindowDTO `json:"window"`
	Group       modelOperationsGroup     `json:"group"`
	Models      []modelOperationsModel   `json:"models"`
	Warning     string                   `json:"warning,omitempty"`
}

type modelOperationsWindowDTO struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type modelOperationsGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type modelOperationsModel struct {
	ModelID               string   `json:"model_id"`
	PublicID              string   `json:"public_id"`
	ActualPlatform        string   `json:"actual_platform"`
	DiscoverySource       string   `json:"discovery_source"`
	Configured            bool     `json:"configured"`
	Discovered            bool     `json:"discovered"`
	CatalogMember         bool     `json:"catalog_member"`
	Routable              *bool    `json:"routable,omitempty"`
	CurrentAvailability   string   `json:"current_availability"`
	RateLimitedOrCooldown *bool    `json:"rate_limited_or_cooldown,omitempty"`
	Healthy               *bool    `json:"healthy,omitempty"`
	V1ModelsVisible       bool     `json:"v1_models_visible"`
	CodexPickerVisible    bool     `json:"codex_picker_visible"`
	RouteType             string   `json:"route_type"`
	AvailableRouteCount   *int     `json:"available_route_count,omitempty"`
	RecentRequestCount    int64    `json:"recent_request_count"`
	RecentInputTokens     int64    `json:"recent_input_tokens"`
	RecentOutputTokens    int64    `json:"recent_output_tokens"`
	RecentTotalTokens     int64    `json:"recent_total_tokens"`
	SampleCount           int64    `json:"sample_count"`
	LatencyP50Ms          *int64   `json:"latency_p50_ms"`
	LatencyP95Ms          *int64   `json:"latency_p95_ms"`
	TTFTP50Ms             *int64   `json:"ttft_p50_ms"`
	TTFTP95Ms             *int64   `json:"ttft_p95_ms"`
	OutputTokensPerSecond *float64 `json:"output_tokens_per_second"`
	TimingSampleCount     int64    `json:"timing_sample_count"`
}

// GetModelOperations returns an admin-only, passive model snapshot for one
// group. It does not probe providers or generate traffic.
func (h *GroupHandler) GetModelOperations(c *gin.Context) {
	groupID, ok := parsePositiveIDParam(c, "id")
	if !ok {
		return
	}
	group, err := h.adminService.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	now := time.Now().UTC()
	start := now.Add(-modelOperationsWindow)
	usage := make(map[string]usagestats.ModelStat)
	warning := ""
	if h.dashboardService != nil {
		stats, statsErr := h.dashboardService.GetModelStatsWithFiltersBySource(
			c.Request.Context(), start, now, 0, 0, 0, groupID, nil, nil, nil, usagestats.ModelSourceRequested,
		)
		if statsErr != nil {
			warning = "Recent passive usage is unavailable."
		} else {
			for _, stat := range stats {
				usage[stat.Model] = stat
			}
		}
	}

	modelsByID := make(map[string]modelOperationsModel)
	if h.gatewayService != nil {
		capabilities := h.gatewayService.BuildGatewayCapabilityModels(
			c.Request.Context(), group, service.DefaultGatewayCapabilityFallbacks(),
		)
		for _, capability := range capabilities {
			limited := capability.RateLimitedOrCooldown
			row := modelOperationsModel{
				ModelID:               capability.ID,
				PublicID:              capability.ID,
				ActualPlatform:        capability.ActualPlatform,
				DiscoverySource:       capability.DiscoverySource,
				Configured:            capability.Configured || capability.DiscoverySource == "explicit_route",
				Discovered:            capability.Discovered,
				CatalogMember:         true,
				Routable:              capability.Routing.Routable,
				CurrentAvailability:   capability.Availability,
				RateLimitedOrCooldown: &limited,
				Healthy:               modelOperationsHealth(capability.Availability),
				V1ModelsVisible:       true,
				CodexPickerVisible:    len(service.FilterCodexModelIDsForGroup([]string{capability.ID}, group)) == 1,
				RouteType:             capability.Routing.Type,
				AvailableRouteCount:   capability.Routing.CandidatePaths,
			}
			applyModelOperationsUsage(&row, usage[capability.ID])
			modelsByID[capability.ID] = row
		}
	}

	models := make([]modelOperationsModel, 0, len(modelsByID))
	for _, model := range modelsByID {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].PublicID < models[j].PublicID })
	response.Success(c, modelOperationsResponse{
		GeneratedAt: now.Format(time.RFC3339Nano),
		Window:      modelOperationsWindowDTO{Start: start.Format(time.RFC3339), End: now.Format(time.RFC3339)},
		Group:       modelOperationsGroup{ID: group.ID, Name: group.Name, Platform: group.Platform},
		Models:      models,
		Warning:     warning,
	})
}

func modelOperationsHealth(availability string) *bool {
	switch availability {
	case service.GatewayAvailabilityAvailable, service.GatewayAvailabilityDegraded:
		value := true
		return &value
	case service.GatewayAvailabilityUnavailable:
		value := false
		return &value
	default:
		return nil
	}
}

func applyModelOperationsUsage(model *modelOperationsModel, stat usagestats.ModelStat) {
	model.RecentRequestCount = stat.Requests
	model.RecentInputTokens = stat.InputTokens
	model.RecentOutputTokens = stat.OutputTokens
	model.RecentTotalTokens = stat.TotalTokens
	model.SampleCount = stat.Requests
	model.LatencyP50Ms = stat.LatencyP50Ms
	model.LatencyP95Ms = stat.LatencyP95Ms
	model.TTFTP50Ms = stat.TTFTP50Ms
	model.TTFTP95Ms = stat.TTFTP95Ms
	model.OutputTokensPerSecond = stat.OutputTokensPerSecond
	model.TimingSampleCount = stat.TimingSampleCount
}
