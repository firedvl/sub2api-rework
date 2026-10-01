//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceHandlerLifecycleAndOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	var owner int64
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		body := `{"id":"task-ark","status":"queued"}`
		if req.Method == http.MethodPost {
			owner = id
		} else {
			require.Equal(t, owner, id)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	newContext := func(method string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), method == http.MethodPost)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		body := ""
		if method == http.MethodPost {
			body = `{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`
		}
		c.Request = httptest.NewRequest(method, "/api/v3/contents/generations/tasks", strings.NewReader(body))
		c.Params = gin.Params{{Key: "task_id", Value: "task-ark"}}
		return c, w
	}
	c, w := newContext(http.MethodPost)
	h.SeedanceTasks(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Positive(t, owner)
	require.Len(t, bindings.pending, 1)
	slots.assertReleased(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method)
		h.SeedanceTasks(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		slots.assertReleased(t)
	}
	for _, other := range []string{"user", "key", "group", "task", "provider"} {
		c, w = newContext(http.MethodGet)
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "key":
			key.ID = 21
		case "group":
			group := int64(25)
			key.GroupID = &group
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other"}}
		case "provider":
			c.Params = gin.Params{{Key: "request_id", Value: "task-ark"}}
		}
		before := upstream.calls
		if other == "provider" {
			h.GrokVideoStatus(c)
		} else {
			h.SeedanceTasks(c)
		}
		require.Equal(t, 404, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	c, _ = newContext(http.MethodGet)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}, ResponseID: "seedance:task-ark"}
	for i := range 20 {
		billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, result.ResponseID, result)
		if i == 0 {
			require.NotNil(t, billed)
			require.Equal(t, "doubao-seedance", billed.BillingModel)
			require.Equal(t, 12345, billed.Usage.OutputTokens)
			require.Zero(t, billed.VideoCount)
		} else {
			require.Nil(t, billed)
		}
	}
	require.Len(t, bindings.billed, 1)
}

func TestSeedanceSimpleModeAllowsUngroupedKey(t *testing.T) {
	handler, slots, _, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	upstream.call = func(*http.Request, int64) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"ungrouped-task","status":"queued"}`))}, nil
	}
	request, recorder := grokMediaSlotContext(context.Background(), true)
	key, ok := middleware.GetAPIKeyFromContext(request)
	require.True(t, ok)
	key.Group = nil
	key.GroupID = nil
	request.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{"model":"configured-video","content":[{"type":"text","text":"waves"}]}`))
	request.Request.Header.Set("Content-Type", "application/json")
	handler.SeedanceTasks(request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, upstream.calls)
	slots.assertReleased(t)
}

func TestSeedanceCompletionKeepsCreateTimeBillingAndPublicModel(t *testing.T) {
	handler, _, bindings, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	request, _ := grokMediaSlotContext(context.Background(), false)
	key, ok := middleware.GetAPIKeyFromContext(request)
	require.True(t, ok)
	subject, ok := middleware.GetAuthSubjectFromContext(request)
	require.True(t, ok)
	taskID := service.SeedanceTaskKey("aliased-task")
	require.NoError(t, handler.gatewayService.StoreGrokVideoPendingBilling(context.Background(), taskID, subject.UserID, key.ID, service.GrokVideoPendingBilling{
		Model: "channel-video", BillingModel: "channel-video", OriginalModel: "public-video", UpstreamModel: "ep-fixture",
		CreatedAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
	}))
	result := prepareSeedanceCompletionBilling(context.Background(), handler, key, subject, taskID, &service.OpenAIForwardResult{
		Usage: service.OpenAIUsage{OutputTokens: 12345}, UpstreamModel: "provider-response-alias", ResponseID: taskID,
	})
	require.NotNil(t, result)
	require.Equal(t, "public-video", result.Model)
	require.Equal(t, "channel-video", result.BillingModel)
	require.Equal(t, "ep-fixture", result.UpstreamModel)
	require.Equal(t, service.StableGrokVideoBillingRequestID(taskID), result.RequestID)
	require.GreaterOrEqual(t, result.Duration, 59*time.Second)
	require.Zero(t, result.VideoCount)
	require.Len(t, bindings.billed, 1)
}

func TestSeedanceLookupRequiresCreateTimeModelSnapshot(t *testing.T) {
	handler, slots, _, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	groupID := int64(24)
	require.NoError(t, handler.gatewayService.BindGrokMediaVideoRequestAccount(context.Background(), &groupID, service.SeedanceTaskKey("task"), 10, 20, 1))
	request, recorder := grokMediaSlotContext(context.Background(), false)
	request.Params = gin.Params{{Key: "task_id", Value: "task"}}
	handler.SeedanceTasks(request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Zero(t, upstream.calls)
	slots.assertReleased(t)
}
