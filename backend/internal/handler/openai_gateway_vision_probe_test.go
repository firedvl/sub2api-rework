package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type visionProbeHandlerAccountRepo struct {
	service.AccountRepository
	consumeCalled bool
}

func (r *visionProbeHandlerAccountRepo) ConsumeOpenAIVisionProbeCandidate(context.Context, int64, *int64) (bool, error) {
	r.consumeCalled = true
	return true, nil
}

func TestOpenAIResponsesVisionProbeRejectsArbitraryImageBeforeCandidateClaim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	repo := &visionProbeHandlerAccountRepo{}
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	gateway := service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billingCache, nil,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
	)
	handler := NewOpenAIGatewayHandler(
		gateway, service.NewConcurrencyService(nil), billingCache,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil, nil, nil, nil, cfg,
	)

	groupID := int64(7)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(
		`{"model":"gpt-5.6-sol","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,cm9ibG94"}]}]}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(service.OpenAIVisionProbeContractHeader, service.OpenAIVisionProbeContract)
	c.Request.Header.Set(service.OpenAIVisionProbeAccountHeader, "31")
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID: 11, GroupID: &groupID,
		User:  &service.User{ID: 13, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 13, Concurrency: 0})

	handler.Responses(c)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "HOSTED_VISION_UNAVAILABLE", gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
	require.False(t, repo.consumeCalled)
}
