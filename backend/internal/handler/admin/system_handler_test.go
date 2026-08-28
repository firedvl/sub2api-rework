//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type systemWatcherStub struct{ info *service.UpdateInfo }

func (s *systemWatcherStub) CheckUpdate(context.Context, bool) (*service.UpdateInfo, error) {
	return s.info, nil
}

type systemUpdaterStub struct {
	status  *updatecontract.UpdaterStatus
	request updatecontract.OperationRequest
	action  updatecontract.Operation
}

func (s *systemUpdaterStub) Status(context.Context) (*updatecontract.UpdaterStatus, error) {
	return s.status, nil
}
func (s *systemUpdaterStub) accepted(action updatecontract.Operation, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	s.action, s.request = action, request
	return &updatecontract.OperationAccepted{OperationID: "op-1", Action: action, State: "accepted"}, nil
}
func (s *systemUpdaterStub) Prepare(_ context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return s.accepted(updatecontract.OperationPrepare, request)
}
func (s *systemUpdaterStub) Install(_ context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return s.accepted(updatecontract.OperationInstall, request)
}
func (s *systemUpdaterStub) Rollback(_ context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return s.accepted(updatecontract.OperationRollback, request)
}

func readyUpdateInfo() *service.UpdateInfo {
	return &service.UpdateInfo{
		CurrentVersion: "0.1.183-rework.1", LatestCompatibleRework: "0.1.184-rework.1",
		State: service.ReleaseStateUpdateReady, Installable: true,
	}
}

func systemTestRouter(handler *SystemHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	router.GET("/api/v1/admin/system/check-updates", handler.CheckUpdates)
	router.POST("/api/v1/admin/system/prepare", handler.Prepare)
	router.POST("/api/v1/admin/system/install", handler.Install)
	router.POST("/api/v1/admin/system/rollback", handler.Rollback)
	return router
}

func TestSystemHandlerCheckUpdatesIncludesUnavailableUpdater(t *testing.T) {
	handler := NewSystemHandler(&systemWatcherStub{info: readyUpdateInfo()}, &systemUpdaterStub{})
	recorder := httptest.NewRecorder()
	systemTestRouter(handler).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/check-updates", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"state":"update_ready"`)
}

func TestSystemHandlerSurfacesFailedUpdaterAsUpdateFailed(t *testing.T) {
	updater := &systemUpdaterStub{status: &updatecontract.UpdaterStatus{
		SchemaVersion: 1, Healthy: false, State: updatecontract.UpdaterStateCritical,
	}}
	handler := NewSystemHandler(&systemWatcherStub{info: readyUpdateInfo()}, updater)
	recorder := httptest.NewRecorder()
	systemTestRouter(handler).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/check-updates", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"state":"update_failed"`)
	require.Contains(t, recorder.Body.String(), `"installable":false`)
}

func TestSystemHandlerRejectsArbitraryImageField(t *testing.T) {
	updater := &systemUpdaterStub{}
	handler := NewSystemHandler(&systemWatcherStub{info: readyUpdateInfo()}, updater)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/prepare", strings.NewReader(`{"version":"0.1.184-rework.1","image":"attacker/image"}`))
	req.Header.Set("Content-Type", "application/json")
	systemTestRouter(handler).ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, updater.action)
}

func TestSystemHandlerInstallRequiresExactConfirmationAndDerivesActor(t *testing.T) {
	updater := &systemUpdaterStub{}
	handler := NewSystemHandler(&systemWatcherStub{info: readyUpdateInfo()}, updater)
	router := systemTestRouter(handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/install", strings.NewReader(`{"version":"0.1.184-rework.1","confirmation":"yes"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/install", strings.NewReader(`{"version":"0.1.184-rework.1","confirmation":"INSTALL 0.1.184-rework.1"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, updatecontract.OperationInstall, updater.action)
	require.Equal(t, "admin:7", updater.request.Actor)
	require.Equal(t, "0.1.184-rework.1", updater.request.Version)

	var body struct {
		Data updatecontract.OperationAccepted `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "op-1", body.Data.OperationID)
}

func TestSystemHandlerRollbackOnlyAllowsRecordedTarget(t *testing.T) {
	updater := &systemUpdaterStub{status: &updatecontract.UpdaterStatus{RollbackVersion: "0.1.183-rework.1"}}
	handler := NewSystemHandler(&systemWatcherStub{info: readyUpdateInfo()}, updater)
	router := systemTestRouter(handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/rollback", strings.NewReader(`{"version":"0.1.182-rework.1","confirmation":"ROLLBACK 0.1.182-rework.1"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, updater.action)
}
