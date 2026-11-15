package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// anthropicUsageClaims maps a provider usage window to the value Anthropic
// sends in anthropic-ratelimit-unified-representative-claim for it. Claude
// Code treats a 429 as a plan usage limit, rather than a temporary throttle,
// only when that header is present.
var anthropicUsageClaims = map[string]string{
	"5h": "five_hour",
	"7d": "seven_day",
}

// usageLimitClassification builds the 429 returned when every account that
// could serve the request is out of usage until a known time.
func usageLimitClassification(limit service.UsageExhaustion, now time.Time) noAccountErrorClassification {
	return noAccountErrorClassification{
		Status:  http.StatusTooManyRequests,
		ErrType: "rate_limit_error",
		Message: fmt.Sprintf(
			"Usage limit reached: all accounts that can serve this model are out of usage. Resets at %s (in %s).",
			limit.ResetAt.UTC().Format(time.RFC3339),
			(time.Duration(usageLimitRetryAfterSeconds(limit.ResetAt, now)) * time.Second).String(),
		),
		UsageLimit: &limit,
	}
}

func usageLimitRetryAfterSeconds(resetAt, now time.Time) int {
	return max(1, int(math.Ceil(resetAt.Sub(now).Seconds())))
}

// setUsageLimitHeaders tells the client when usage comes back. Every protocol
// gets Retry-After in integer seconds. The Anthropic Messages protocol also
// gets the unified usage headers Claude Code reads, with the window-specific
// ones only when the binding window is known. Headers are set before any body
// is written; once a stream has started they are no longer sent.
func setUsageLimitHeaders(c *gin.Context, limit *service.UsageExhaustion) {
	if c == nil || limit == nil {
		return
	}
	c.Header("Retry-After", strconv.Itoa(usageLimitRetryAfterSeconds(limit.ResetAt, time.Now())))
	if GetInboundEndpoint(c) != EndpointMessages {
		return
	}
	reset := strconv.FormatInt(limit.ResetAt.Unix(), 10)
	c.Header("anthropic-ratelimit-unified-status", "rejected")
	c.Header("anthropic-ratelimit-unified-reset", reset)
	claim, ok := anthropicUsageClaims[limit.Window]
	if !ok {
		return
	}
	c.Header("anthropic-ratelimit-unified-representative-claim", claim)
	c.Header("anthropic-ratelimit-unified-"+limit.Window+"-status", "rejected")
	c.Header("anthropic-ratelimit-unified-"+limit.Window+"-reset", reset)
}

// writeOpenAIUsageLimitError writes the usage-limit body used on the OpenAI
// Responses and Chat Completions protocols. It is the ChatGPT backend shape
// that Codex CLI maps to its usage-limit error (error.type plus resets_at) and
// that this gateway's own upstream 429 parser reads.
func writeOpenAIUsageLimitError(c *gin.Context, cls noAccountErrorClassification) {
	resetAt := cls.UsageLimit.ResetAt
	c.JSON(cls.Status, gin.H{
		"error": gin.H{
			"type":              "usage_limit_reached",
			"message":           cls.Message,
			"resets_at":         resetAt.Unix(),
			"resets_in_seconds": usageLimitRetryAfterSeconds(resetAt, time.Now()),
		},
	})
}

// classifyUsageLimitAfterUpstream429 covers the request that discovers the
// limit: its upstream 429 marked the last usable account, so failover ran out
// of accounts. ok is true only when the pool is now out of usage until a known
// time; callers otherwise keep their existing failover-exhausted response.
func classifyUsageLimitAfterUpstream429(
	c *gin.Context,
	diag service.ModelAvailabilityDiagnoser,
	apiKey *service.APIKey,
	routingModel string,
	platform string,
	lastErr *service.UpstreamFailoverError,
) (noAccountErrorClassification, bool) {
	if lastErr == nil || lastErr.StatusCode != http.StatusTooManyRequests {
		return noAccountErrorClassification{}, false
	}
	cls := classifyNoAccountError(c.Request.Context(), diag, apiKey, routingModel, routingModel, platform)
	if cls.UsageLimit == nil {
		return noAccountErrorClassification{}, false
	}
	setUsageLimitHeaders(c, cls.UsageLimit)
	service.SetOpsUpstreamError(c, lastErr.StatusCode, service.ExtractUpstreamErrorMessage(lastErr.ResponseBody), "")
	return cls, true
}

// respondUsageLimitAfterUpstream429 writes the Anthropic-format usage-limit
// response for the request that discovered the limit, and reports whether it
// did. An operator passthrough rule for the upstream error keeps precedence.
func (h *GatewayHandler) respondUsageLimitAfterUpstream429(
	c *gin.Context,
	apiKey *service.APIKey,
	routingModel string,
	platform string,
	lastErr *service.UpstreamFailoverError,
	streamStarted bool,
) bool {
	if lastErr != nil && h.errorPassthroughService != nil &&
		h.errorPassthroughService.MatchRule(platform, lastErr.StatusCode, lastErr.ResponseBody) != nil {
		return false
	}
	cls, ok := classifyUsageLimitAfterUpstream429(c, h.gatewayService, apiKey, routingModel, platform, lastErr)
	if !ok {
		return false
	}
	h.handleStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
	return true
}

// respondNoAccount writes a no-account classification on the OpenAI Responses
// and Chat Completions protocols. A usage limit gets its dedicated body unless
// the response is already committed, in which case the existing in-stream
// error path carries the message.
func (h *OpenAIGatewayHandler) respondNoAccount(c *gin.Context, cls noAccountErrorClassification, streamStarted bool) {
	if cls.UsageLimit != nil && !streamStarted && !service.StopOpenAICompactSSEKeepaliveCommitted(c) {
		writeOpenAIUsageLimitError(c, cls)
		return
	}
	h.handleStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
}

// respondUsageLimitAfterUpstream429 is the OpenAI-format counterpart of the
// GatewayHandler method of the same name.
func (h *OpenAIGatewayHandler) respondUsageLimitAfterUpstream429(
	c *gin.Context,
	apiKey *service.APIKey,
	routingModel string,
	platform string,
	lastErr *service.UpstreamFailoverError,
	streamStarted bool,
) bool {
	if lastErr != nil && h.errorPassthroughService != nil &&
		h.errorPassthroughService.MatchRule("openai", lastErr.StatusCode, lastErr.ResponseBody) != nil {
		return false
	}
	cls, ok := classifyUsageLimitAfterUpstream429(c, h.gatewayService, apiKey, routingModel, platform, lastErr)
	if !ok {
		return false
	}
	h.respondNoAccount(c, cls, streamStarted)
	return true
}
