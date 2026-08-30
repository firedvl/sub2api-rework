package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operatorAssistantApplicationStub struct {
	models service.OperatorAssistantModels
	run    service.OperatorAssistantRun
	err    error
	calls  int
}

func (s *operatorAssistantApplicationStub) Models(context.Context) (service.OperatorAssistantModels, error) {
	return s.models, s.err
}

func (s *operatorAssistantApplicationStub) Stream(c *gin.Context, _ int64, _ service.OperatorAssistantRequest) (service.OperatorAssistantRun, error) {
	s.calls++
	if s.err == nil {
		c.Header("Content-Type", "text/event-stream")
		_, _ = c.Writer.WriteString("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
	}
	return s.run, s.err
}

func operatorAssistantHandlerRouter(application operatorAssistantApplication, role string, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if authenticated || role != "" {
		router.Use(func(c *gin.Context) {
			if authenticated {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			}
			if role != "" {
				c.Set(string(middleware.ContextKeyUserRole), role)
			}
			c.Next()
		})
	}
	handler := &OperatorAssistantHandler{service: application}
	router.GET("/models", handler.Models)
	router.POST("/stream", handler.Stream)
	return router
}

func TestOperatorAssistantHandlerDefendsAdminAuthorization(t *testing.T) {
	application := &operatorAssistantApplicationStub{}
	for _, test := range []struct {
		name          string
		role          string
		authenticated bool
		wantStatus    int
	}{
		{name: "missing subject", wantStatus: http.StatusUnauthorized},
		{name: "normal user", role: service.RoleUser, authenticated: true, wantStatus: http.StatusForbidden},
		{name: "admin", role: service.RoleAdmin, authenticated: true, wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/models", nil)
			operatorAssistantHandlerRouter(application, test.role, test.authenticated).ServeHTTP(recorder, request)
			require.Equal(t, test.wantStatus, recorder.Code)
		})
	}
}

func TestOperatorAssistantHandlerStreamsAndRejectsInvalidInput(t *testing.T) {
	application := &operatorAssistantApplicationStub{run: service.OperatorAssistantRun{Started: true}}
	router := operatorAssistantHandlerRouter(application, service.RoleAdmin, true)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/stream", bytes.NewBufferString(`{"model":"auto","messages":[{"role":"user","content":"What needs attention?"}],"route":"/admin/dashboard"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Body.String(), "response.output_text.delta")
	require.Equal(t, 1, application.calls)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/stream", bytes.NewBufferString(`{"messages":[{"role":"tool","content":"mutate"}]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 1, application.calls)
}
