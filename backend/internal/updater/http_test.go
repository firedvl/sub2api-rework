//go:build unit

package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdaterHTTPRejectsUnknownFieldsAndRoutes(t *testing.T) {
	policy := testPolicy(t.TempDir())
	writeUpdaterTestDeployment(t, policy)
	service, err := NewService(policy, &fakeRunner{}, &fakeManifestFetcher{})
	require.NoError(t, err)
	handler := service.Handler()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/install", strings.NewReader(`{"version":"0.1.184-rework.1","confirmation":"INSTALL 0.1.184-rework.1","actor":"admin:1","image":"attacker/image"}`))
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/v1/exec", strings.NewReader(`{"command":"id"}`))
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestServeUnixRejectsWritableSocketParent(t *testing.T) {
	policy := testPolicy(t.TempDir())
	directory := filepath.Dir(policy.SocketPath)
	require.NoError(t, os.MkdirAll(directory, 0750))
	require.NoError(t, os.Chmod(directory, 0770))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ServeUnix(ctx, policy, http.NotFoundHandler())

	require.ErrorContains(t, err, "permissions")
}
