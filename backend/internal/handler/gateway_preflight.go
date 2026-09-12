package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Preflight accepts only request traits, never prompts, credentials or an
// inference payload. It deliberately uses no forwarding or admission handler.
func (h *GatewayHandler) Preflight(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var request service.GatewayPreflightRequest
	err := decoder.Decode(&request)
	if err == nil {
		var extra any
		trailingErr := decoder.Decode(&extra)
		if trailingErr != io.EOF {
			err = trailingErr
			if err == nil {
				err = errors.New("trailing JSON")
			}
		}
	}
	if err != nil {
		status := http.StatusBadRequest
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			status = http.StatusRequestEntityTooLarge
		}
		h.errorResponse(c, status, "invalid_request_error", "Invalid preflight request")
		return
	}
	result, err := h.gatewayService.PreflightGatewayRequest(c.Request.Context(), apiKey.Group, request, h.openAIGatewayService)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}
