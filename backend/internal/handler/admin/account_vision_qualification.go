package admin

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type runOpenAIVisionQualificationRequest struct {
	Stage string `json:"stage"`
}

func (h *AccountHandler) GetOpenAIVisionQualification(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	report, err := h.accountTestService.GetOpenAIVisionQualification(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func (h *AccountHandler) RunOpenAIVisionQualification(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req runOpenAIVisionQualificationRequest
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 1<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	report, err := h.accountTestService.RunOpenAIVisionQualification(c.Request.Context(), accountID, req.Stage)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("request must contain exactly one JSON object")
}

func (h *AccountHandler) PromoteOpenAIVisionQualification(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	report, err := h.accountTestService.PromoteOpenAIVisionQualification(c.Request.Context(), accountID, h.adminService)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}
