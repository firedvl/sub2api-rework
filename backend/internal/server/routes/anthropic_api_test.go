package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicAPIOnlyAdmission(t *testing.T) {
	for _, platform := range []string{"", service.PlatformOpenAI, service.PlatformGemini, service.PlatformComposite, service.PlatformAnthropic} {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: platform}})
			// A prior composite resolution must not authorize a composite group.
			c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformAnthropic))
			c.Next()
		})
		router.POST("/test", anthropicAPIOnly(), func(c *gin.Context) {
			require.True(t, service.IsAnthropicAPIOnly(c.Request.Context()))
			require.Nil(t, c.Request.Context().Value(ctxkey.ForcePlatform))
			c.Status(http.StatusNoContent)
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", nil))
		if platform == service.PlatformAnthropic {
			require.Equal(t, http.StatusNoContent, w.Code)
		} else {
			require.Equal(t, http.StatusForbidden, w.Code)
		}
	}
}

func TestAnthropicAPIRoutesMountModelAdmission(t *testing.T) {
	router := newGatewayRoutesTestRouterWithGroup(allowlistGroup(service.PlatformAnthropic, true, "claude-allowed"))
	for _, endpoint := range []string{"/anthropic-api/v1/messages", "/anthropic-api/v1/messages/count_tokens"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"model":"not-allowed","messages":[]}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Contains(t, w.Body.String(), "not-allowed")
	}
}
