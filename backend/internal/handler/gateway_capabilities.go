package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/releaseinfo"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type gatewayCapabilitiesResponse struct {
	SchemaVersion int                              `json:"schema_version"`
	GeneratedAt   string                           `json:"generated_at"`
	Gateway       gatewayCapabilityIdentity        `json:"gateway"`
	Transport     gatewayCapabilityTransport       `json:"transport"`
	Models        []service.GatewayCapabilityModel `json:"models"`
}

type gatewayCapabilityIdentity struct {
	Version         string `json:"version"`
	UpstreamVersion string `json:"upstream_version"`
}

type gatewayCapabilityTransport struct {
	HTTP            bool `json:"http"`
	Responses       bool `json:"responses"`
	ChatCompletions bool `json:"chat_completions"`
	SSE             bool `json:"sse"`
	WebSocket       bool `json:"websocket"`
}

// Capabilities returns the versioned, API-key-scoped Gateway discovery contract.
// GET /v1/gateway/capabilities
func (h *GatewayHandler) Capabilities(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	var models []service.GatewayCapabilityModel
	if h != nil && h.gatewayService != nil {
		models = h.gatewayService.BuildGatewayCapabilityModels(c.Request.Context(), apiKey.Group, gatewayCapabilityFallbacks())
	} else {
		models = (&service.GatewayService{}).BuildGatewayCapabilityModels(c.Request.Context(), apiKey.Group, gatewayCapabilityFallbacks())
	}
	for i := range models {
		models[i].DisplayName = gatewayCapabilityDisplayName(models[i].ID)
		models[i].Capabilities = gatewayCapabilityMetadata(models[i].ID)
	}

	metadata := releaseinfo.Current()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gatewayCapabilitiesResponse{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Gateway: gatewayCapabilityIdentity{
			Version:         metadata.ReworkVersion,
			UpstreamVersion: metadata.UpstreamBaseline,
		},
		Transport: gatewayCapabilityTransport{
			HTTP:            true,
			Responses:       true,
			ChatCompletions: true,
			SSE:             true,
			// WebSocket is not part of the supported integration contract.
			WebSocket: false,
		},
		Models: models,
	})
}

func gatewayCapabilityFallbacks() map[string][]string {
	return map[string][]string{
		service.PlatformAnthropic:   defaultModelIDsForPlatform(service.PlatformAnthropic),
		service.PlatformGemini:      defaultModelIDsForPlatform(service.PlatformGemini),
		service.PlatformOpenAI:      defaultModelIDsForPlatform(service.PlatformOpenAI),
		service.PlatformAntigravity: defaultModelIDsForPlatform(service.PlatformAntigravity),
		service.PlatformGrok:        defaultModelIDsForPlatform(service.PlatformGrok),
		service.PlatformComposite:   defaultModelIDsForPlatform(service.PlatformComposite),
	}
}

func gatewayCapabilityDisplayName(modelID string) string {
	for _, model := range openai.DefaultModels {
		if model.ID == modelID {
			return model.DisplayName
		}
	}
	for _, model := range claude.DefaultModels {
		if model.ID == modelID {
			return model.DisplayName
		}
	}
	for _, model := range geminicli.DefaultModels {
		if model.ID == modelID {
			return model.DisplayName
		}
	}
	for _, model := range antigravity.DefaultModels() {
		if model.ID == modelID {
			return model.DisplayName
		}
	}
	for _, model := range xai.DefaultModels() {
		if model.ID == modelID {
			return model.DisplayName
		}
	}
	return modelID
}

func gatewayCapabilityMetadata(modelID string) *service.GatewayModelCapabilities {
	metadata := &service.GatewayModelCapabilities{}
	known := false
	if antigravity.IsGeminiReasoningModel(modelID) || grokModelSupportsConfigurableReasoning(modelID) || strings.EqualFold(modelID, "grok-4.20-0309-reasoning") {
		metadata.Reasoning = gatewayCapabilityBool(true)
		known = true
	} else if strings.EqualFold(modelID, "grok-4.20-0309-non-reasoning") {
		metadata.Reasoning = gatewayCapabilityBool(false)
		known = true
	}
	if service.IsGPTImageGenerationModel(modelID) || modelID == xai.DefaultImagineImageQualityModel || modelID == xai.DefaultImagineImageFastModel || modelID == xai.DefaultImagineImage20Model {
		metadata.ImageOutput = gatewayCapabilityBool(true)
		known = true
	}
	if !known {
		return nil
	}
	return metadata
}

func gatewayCapabilityBool(value bool) *bool { return &value }
