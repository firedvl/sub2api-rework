package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/Wei-Shaw/sub2api/internal/updaterclient"

	"github.com/gin-gonic/gin"
)

const maxUpdaterRequestBytes = 4 * 1024

type systemReleaseWatcher interface {
	CheckUpdate(ctx context.Context, force bool) (*service.UpdateInfo, error)
}

type systemUpdater interface {
	Status(ctx context.Context) (*updatecontract.UpdaterStatus, error)
	Prepare(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error)
	Install(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error)
	Rollback(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error)
}

type SystemHandler struct {
	watcher systemReleaseWatcher
	updater systemUpdater
}

func NewSystemHandler(watcher systemReleaseWatcher, updater systemUpdater) *SystemHandler {
	return &SystemHandler{watcher: watcher, updater: updater}
}

func (h *SystemHandler) GetVersion(c *gin.Context) {
	info, err := h.watcher.CheckUpdate(c.Request.Context(), false)
	if err != nil || info == nil {
		response.InternalError(c, "Failed to read release metadata")
		return
	}
	response.Success(c, gin.H{
		"version": info.CurrentVersion, "git_commit": info.CurrentGitCommit,
		"build_date": info.BuildDate, "build_type": info.BuildType,
		"upstream_baseline": info.UpstreamBaseline, "upstream_baseline_sha": info.UpstreamBaselineSHA,
		"update_channel": info.UpdateChannel, "update_policy": info.UpdatePolicy,
	})
}

func (h *SystemHandler) CheckUpdates(c *gin.Context) {
	info, err := h.watcher.CheckUpdate(c.Request.Context(), c.Query("force") == "true")
	if err != nil || info == nil {
		response.InternalError(c, "Failed to check releases")
		return
	}
	updaterStatus := h.updaterStatus(c.Request.Context())
	combined := *info
	if updaterStatus.State == updatecontract.UpdaterStateFailed || updaterStatus.State == updatecontract.UpdaterStateCritical {
		combined.State = service.ReleaseStateUpdateFailed
		combined.Installable = false
	}
	response.Success(c, struct {
		*service.UpdateInfo
		Updater updatecontract.UpdaterStatus `json:"updater"`
	}{UpdateInfo: &combined, Updater: updaterStatus})
}

func (h *SystemHandler) Prepare(c *gin.Context) {
	req, ok := decodeUpdaterRequest(c)
	if !ok || !h.requireReadyRelease(c, req.Version) {
		return
	}
	middleware2.SetAuditAction(c, "admin.system.update.prepare")
	h.start(c, updatecontract.OperationPrepare, req, func(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
		return h.updater.Prepare(ctx, request)
	})
}

func (h *SystemHandler) Install(c *gin.Context) {
	req, ok := decodeUpdaterRequest(c)
	if !ok || !h.requireReadyRelease(c, req.Version) {
		return
	}
	if req.Confirmation != "INSTALL "+req.Version {
		response.BadRequest(c, "Explicit install confirmation does not match the requested version")
		return
	}
	middleware2.SetAuditAction(c, "admin.system.update.install")
	h.start(c, updatecontract.OperationInstall, req, func(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
		return h.updater.Install(ctx, request)
	})
}

func (h *SystemHandler) Rollback(c *gin.Context) {
	req, ok := decodeUpdaterRequest(c)
	if !ok {
		return
	}
	status, err := h.updater.Status(c.Request.Context())
	if err != nil {
		h.updaterError(c, err)
		return
	}
	if status == nil || status.RollbackVersion == "" || req.Version != status.RollbackVersion {
		response.BadRequest(c, "Requested version is not the updater's recorded rollback target")
		return
	}
	if req.Confirmation != "ROLLBACK "+req.Version {
		response.BadRequest(c, "Explicit rollback confirmation does not match the recorded target")
		return
	}
	middleware2.SetAuditAction(c, "admin.system.update.rollback")
	h.start(c, updatecontract.OperationRollback, req, func(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
		return h.updater.Rollback(ctx, request)
	})
}

type updaterWebRequest struct {
	Version      string `json:"version"`
	Confirmation string `json:"confirmation,omitempty"`
}

func decodeUpdaterRequest(c *gin.Context) (updaterWebRequest, bool) {
	var request updaterWebRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUpdaterRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		response.BadRequest(c, "Invalid updater request")
		return request, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid updater request")
		return request, false
	}
	request.Version = strings.TrimSpace(request.Version)
	if !updatecontract.IsReworkVersion(request.Version) {
		response.BadRequest(c, "Invalid rework version")
		return request, false
	}
	return request, true
}

func (h *SystemHandler) requireReadyRelease(c *gin.Context, version string) bool {
	info, err := h.watcher.CheckUpdate(c.Request.Context(), false)
	if err != nil || info == nil {
		response.InternalError(c, "Failed to verify the approved release")
		return false
	}
	if !info.Installable || info.State != service.ReleaseStateUpdateReady || info.LatestCompatibleRework != version {
		response.Error(c, http.StatusConflict, "Requested release is not approved and ready")
		return false
	}
	return true
}

func (h *SystemHandler) start(
	c *gin.Context,
	action updatecontract.Operation,
	request updaterWebRequest,
	call func(context.Context, updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error),
) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Administrator session required")
		return
	}
	accepted, err := call(c.Request.Context(), updatecontract.OperationRequest{
		Version: request.Version, Confirmation: request.Confirmation,
		Actor: fmt.Sprintf("admin:%d", subject.UserID),
	})
	if err != nil {
		h.updaterError(c, err)
		return
	}
	middleware2.SetAuditExtra(c, map[string]any{
		"action": string(action), "target_version": request.Version, "operation_id": accepted.OperationID,
	})
	response.Accepted(c, accepted)
}

func (h *SystemHandler) updaterStatus(ctx context.Context) updatecontract.UpdaterStatus {
	status, err := h.updater.Status(ctx)
	if err == nil && status != nil {
		return *status
	}
	return updatecontract.UpdaterStatus{
		SchemaVersion: 1, Healthy: false, State: updatecontract.UpdaterStateUnavailable,
		LastError: "Updater service is unavailable.", UpdatedAt: time.Now().UTC(),
	}
}

func (h *SystemHandler) updaterError(c *gin.Context, err error) {
	if errors.Is(err, updaterclient.ErrUnavailable) {
		response.Error(c, http.StatusServiceUnavailable, "Updater service is unavailable")
		return
	}
	response.Error(c, http.StatusConflict, "Updater rejected the operation")
}
