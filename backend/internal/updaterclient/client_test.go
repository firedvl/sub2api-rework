//go:build unit

package updaterclient

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
	"github.com/stretchr/testify/require"
)

func TestClientRejectsTrailingAndOversizedResponses(t *testing.T) {
	for name, body := range map[string]string{
		"trailing":  `{"schema_version":1} {}`,
		"oversized": `{"padding":"` + strings.Repeat("x", maxResponseBytes) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			socket := startUnixUpdater(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			_, err := NewForSocket(socket).Status(context.Background())
			require.ErrorContains(t, err, "invalid updater response")
		})
	}
}

func startUnixUpdater(t *testing.T, handler http.Handler) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "updater.sock")
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
		_ = os.Remove(path)
	})
	return path
}

func TestClientUsesOnlyFixedUnixSocketRoutes(t *testing.T) {
	var gotPath string
	socket := startUnixUpdater(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var request updatecontract.OperationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "0.1.184-rework.1", request.Version)
		require.Equal(t, "admin:7", request.Actor)
		_ = json.NewEncoder(w).Encode(updatecontract.OperationAccepted{OperationID: "op-1", Action: updatecontract.OperationPrepare, State: "accepted"})
	}))
	client := NewForSocket(socket)
	accepted, err := client.Prepare(context.Background(), updatecontract.OperationRequest{Version: "0.1.184-rework.1", Actor: "admin:7"})
	require.NoError(t, err)
	require.Equal(t, "/v1/prepare", gotPath)
	require.Equal(t, "op-1", accepted.OperationID)
}

func TestClientReportsMissingOrInvalidSocketAsUnavailable(t *testing.T) {
	_, err := NewForSocket(filepath.Join(t.TempDir(), "missing.sock")).Status(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = NewForSocket("relative.sock").Status(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestClientRecoveryUsesSeparateRoutes(t *testing.T) {
	var paths []string
	socket := startUnixUpdater(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(updatecontract.RecoveryStatus{SourceVersion: "0.1.183-rework.1", Confirmation: "operation-bound-consent"})
			return
		}
		var request updatecontract.OperationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "PREPARE RECOVERY 0.1.183-rework.1", request.Confirmation)
		_ = json.NewEncoder(w).Encode(updatecontract.OperationAccepted{Action: updatecontract.OperationPrepareRecovery})
	}))
	client := NewForSocket(socket)
	status, err := client.RecoveryStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, "operation-bound-consent", status.Confirmation)
	_, err = client.PrepareRecovery(context.Background(), updatecontract.OperationRequest{Version: status.SourceVersion, Confirmation: "PREPARE RECOVERY 0.1.183-rework.1"})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /v1/recovery", "POST /v1/prepare_recovery"}, paths)
}
