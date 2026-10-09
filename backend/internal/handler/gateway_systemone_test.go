package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const validSystemOneHandlerBody = `{"model":"jev-latest","state":"sample","questions":{"q":{"type":"noul","instructions":"Evaluate"}}}`

func TestSystemOneUsageSnapshotsModelBeforeContextReuse(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	pool.Submit(func(context.Context) { close(started); <-release })
	<-started
	t.Cleanup(func() { unblock(); pool.Stop() })

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	repo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 1)}
	gateway := service.NewGatewayService(
		nil, nil, repo, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, nil, nil, nil,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := &GatewayHandler{gatewayService: gateway, usageRecordWorkerPool: pool}
	c, _ := newSystemOneHandlerContext(validSystemOneHandlerBody)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestedPublicModel, "first-caller-model"))
	key := &service.APIKey{ID: 4, UserID: 5, User: &service.User{ID: 5}}
	account := &service.Account{ID: 6, Platform: service.PlatformTypeSafe}
	result := &service.SystemOneForwardResult{ForwardResult: service.ForwardResult{Model: "jev-latest", UpstreamModel: "jev-latest"}}
	h.recordSystemOneUsage(c, key, account, nil, service.ChannelMappingResult{}, "jev-latest", []byte(validSystemOneHandlerBody), result, 5, time.Now())

	// Gin reuses the same context after the first handler returns.
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestedPublicModel, "later-caller-model"))
	select {
	case <-repo.created:
		t.Fatal("billing ran while the worker was blocked")
	default:
	}
	unblock()
	select {
	case usage := <-repo.created:
		require.Equal(t, "first-caller-model", usage.RequestedModel)
		require.Equal(t, int64(5), usage.UserID)
		require.Equal(t, int64(4), usage.APIKeyID)
		require.Equal(t, int64(6), usage.AccountID)
	case <-time.After(5 * time.Second):
		t.Fatal("queued usage was not recorded")
	}
}

func newSystemOneHandlerContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestSystemOneRequiresAuthentication(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	(&GatewayHandler{}).SystemOne(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "authentication_error")
}

func TestSystemOnePromptGuardBlocksBeforeSchedulingAndBilling(t *testing.T) {
	engine := blockingHandlerPromptEngine()
	c, recorder := newTypeSafeGroupContext(t, "/v1/systemone", validSystemOneHandlerBody, service.PlatformTypeSafe)
	h := &GatewayHandler{
		cfg:                      &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
		securityAuditCoordinator: securityaudit.NewCoordinator(nil, engine),
	}
	h.SystemOne(c)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	evaluated, _, requests := engine.snapshot()
	require.Equal(t, 1, evaluated)
	require.Len(t, requests, 1)
	require.Equal(t, service.ContentModerationProtocolTypeSafeSystemOne, requests[0].Protocol)
	snapshot, err := securityaudit.ExtractPromptSnapshot(requests[0])
	require.NoError(t, err)
	require.Contains(t, snapshot.ScanText, "sample")
	require.Contains(t, snapshot.ScanText, "Evaluate")
}

func TestSystemOneRejectsNonTypeSafeGroupBeforeScheduling(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	groupID := int64(3)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})

	(&GatewayHandler{cfg: &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}}).SystemOne(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "only available for TypeSafe")
}

func newTypeSafeGroupContext(t *testing.T, path, body, groupPlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newSystemOneHandlerContext(body)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(9)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: groupPlatform}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})
	return c, recorder
}

func TestRejectSystemOneOnlyPlatform(t *testing.T) {
	write := func(c *gin.Context, status int, errType, message string) {
		c.JSON(status, gin.H{"type": errType, "message": message})
	}

	c, recorder := newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformTypeSafe)
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformComposite)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformTypeSafe))
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformAnthropic)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/antigravity/v1/messages", `{}`, service.PlatformTypeSafe)
	c.Set(string(middleware2.ContextKeyForcePlatform), service.PlatformAntigravity)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
}

func mustAPIKey(t *testing.T, c *gin.Context) *service.APIKey {
	t.Helper()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	return apiKey
}

func TestTypeSafeGroupsRejectNonSystemOneProtocolsBeforeScheduling(t *testing.T) {
	h := &GatewayHandler{
		cfg:            &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
		gatewayService: &service.GatewayService{},
	}
	for _, tc := range []struct {
		name    string
		path    string
		body    string
		handler func(*gin.Context)
	}{
		{"messages", "/v1/messages", `{"model":"jev-latest","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, h.Messages},
		{"count_tokens", "/v1/messages/count_tokens", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.CountTokens},
		{"chat_completions", "/v1/chat/completions", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.ChatCompletions},
		{"responses", "/v1/responses", `{"model":"jev-latest","input":"hi"}`, h.Responses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newTypeSafeGroupContext(t, tc.path, tc.body, service.PlatformTypeSafe)
			tc.handler(c)
			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)
		})
	}
}
