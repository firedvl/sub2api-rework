package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicCreditsErrorsAcrossProtocols(t *testing.T) {
	h := &GatewayHandler{}
	handlers := map[string]func(*gin.Context, *service.UpstreamFailoverError, bool){
		"messages": func(c *gin.Context, e *service.UpstreamFailoverError, stream bool) {
			h.handleFailoverExhausted(c, e, service.PlatformAnthropic, stream)
		},
		"responses":             h.handleResponsesFailoverExhausted,
		"chat completions":      h.handleCCFailoverExhausted,
		"openai native adapter": (&OpenAIGatewayHandler{}).handleFailoverExhausted,
	}
	for name, handle := range handlers {
		for _, tc := range []struct {
			name, body, hint string
			status           int
		}{
			{"long context", `{"error":{"type":"rate_limit_error","message":"Usage credits are required for long context requests."}}`, "Compact the conversation", http.StatusBadRequest},
			{"model entitlement", `{"error":{"details":{"error_code":"credits_required","model":"claude-opus-5"},"message":"Usage credits are required for this model."}}`, "Select another model", http.StatusPaymentRequired},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformAnthropic))
				handle(c, &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests, ResponseBody: []byte(tc.body)}, false)
				require.Equal(t, tc.status, w.Code)
				require.Contains(t, w.Body.String(), tc.hint)
				require.NotContains(t, w.Body.String(), "retry later")
			})
		}
	}
}
