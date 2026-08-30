package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const operatorAssistantMaxRequestBytes = 64 * 1024

type operatorAssistantApplication interface {
	Models(context.Context) (service.OperatorAssistantModels, error)
	Stream(*gin.Context, int64, service.OperatorAssistantRequest) (service.OperatorAssistantRun, error)
}

type OperatorAssistantHandler struct {
	service operatorAssistantApplication
}

func NewOperatorAssistantHandler(assistant *service.OperatorAssistantService) *OperatorAssistantHandler {
	return &OperatorAssistantHandler{service: assistant}
}

func (h *OperatorAssistantHandler) Models(c *gin.Context) {
	if _, ok := requireOperatorAssistantAdmin(c); !ok {
		return
	}
	models, err := h.service.Models(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Ask Gateway model discovery is unavailable")
		return
	}
	response.Success(c, models)
}

func (h *OperatorAssistantHandler) Stream(c *gin.Context) {
	subject, ok := requireOperatorAssistantAdmin(c)
	if !ok {
		return
	}
	request, ok := decodeOperatorAssistantRequest(c)
	if !ok {
		return
	}
	if err := service.ValidateOperatorAssistantRequest(request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	middleware2.SetAuditAction(c, "admin.operator_assistant.query")
	startedAt := time.Now()
	run, err := h.service.Stream(c, subject.UserID, request)
	if err == nil {
		middleware2.SetAuditExtra(c, map[string]any{
			"result": "success", "latency_ms": time.Since(startedAt).Milliseconds(),
		})
		return
	}
	middleware2.SetAuditExtra(c, map[string]any{
		"result": "failed", "latency_ms": time.Since(startedAt).Milliseconds(),
	})
	if run.Started || c.Writer.Written() || errors.Is(err, context.Canceled) {
		return
	}
	switch {
	case errors.Is(err, service.ErrOperatorAssistantNoModels):
		response.Error(c, http.StatusServiceUnavailable, "No routable text models are currently available")
	case errors.Is(err, service.ErrOperatorAssistantModelUnavailable):
		response.Error(c, http.StatusConflict, "The selected model is no longer available")
	case errors.Is(err, service.ErrOperatorAssistantCapacityUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "All eligible model capacity is currently busy or limited")
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(c, http.StatusGatewayTimeout, "Ask Gateway timed out")
	default:
		response.Error(c, http.StatusBadGateway, "Ask Gateway could not complete the request")
	}
}

func requireOperatorAssistantAdmin(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Administrator session required")
		return middleware2.AuthSubject{}, false
	}
	role, ok := middleware2.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.Forbidden(c, "Administrator access required")
		return middleware2.AuthSubject{}, false
	}
	return subject, true
}

func decodeOperatorAssistantRequest(c *gin.Context) (service.OperatorAssistantRequest, bool) {
	var request service.OperatorAssistantRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, operatorAssistantMaxRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		response.BadRequest(c, "Invalid Ask Gateway request")
		return request, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid Ask Gateway request")
		return request, false
	}
	return request, true
}
