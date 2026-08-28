package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const maxOperationRequestBytes = 4 * 1024

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", s.handleStatus)
	mux.HandleFunc("POST /v1/prepare", s.handleOperation(updatecontract.OperationPrepare))
	mux.HandleFunc("POST /v1/install", s.handleOperation(updatecontract.OperationInstall))
	mux.HandleFunc("POST /v1/rollback", s.handleOperation(updatecontract.OperationRollback))
	return http.MaxBytesHandler(mux, maxOperationRequestBytes)
}

func (s *Service) handleStatus(w http.ResponseWriter, _ *http.Request) {
	status, err := s.Status()
	if err != nil {
		writeUpdaterError(w, http.StatusInternalServerError, "updater state is unavailable")
		return
	}
	writeUpdaterJSON(w, http.StatusOK, status)
}

func (s *Service) handleOperation(action updatecontract.Operation) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var operation updatecontract.OperationRequest
		if err := decoder.Decode(&operation); err != nil {
			writeUpdaterError(w, http.StatusBadRequest, "invalid updater request")
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			writeUpdaterError(w, http.StatusBadRequest, "invalid updater request")
			return
		}
		accepted, err := s.Start(action, operation)
		if err != nil {
			switch {
			case errors.Is(err, ErrOperationBusy):
				writeUpdaterError(w, http.StatusConflict, "another updater operation is running")
			case errors.Is(err, context.DeadlineExceeded):
				writeUpdaterError(w, http.StatusServiceUnavailable, "updater operation could not start")
			default:
				writeUpdaterError(w, http.StatusBadRequest, "updater request rejected")
			}
			return
		}
		writeUpdaterJSON(w, http.StatusAccepted, accepted)
	}
}

func writeUpdaterJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeUpdaterError(w http.ResponseWriter, status int, message string) {
	writeUpdaterJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func ServeUnix(ctx context.Context, policy Policy, handler http.Handler) error {
	if err := ensureManagedDirectory(filepath.Dir(policy.SocketPath), 0750, false); err != nil {
		return fmt.Errorf("create updater socket directory: %w", err)
	}
	if info, err := os.Lstat(policy.SocketPath); err == nil {
		if err := validateUpdaterSocket(info); err != nil {
			return err
		}
		if err := os.Remove(policy.SocketPath); err != nil {
			return fmt.Errorf("remove stale updater socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.Listen("unix", policy.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on updater socket: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(policy.SocketPath)
	}()
	if err := os.Chmod(policy.SocketPath, 0660); err != nil {
		return fmt.Errorf("set updater socket permissions: %w", err)
	}
	info, err := os.Lstat(policy.SocketPath)
	if err != nil {
		return fmt.Errorf("stat updater socket: %w", err)
	}
	if err := validateUpdaterSocket(info); err != nil {
		return err
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

func validateUpdaterSocket(info os.FileInfo) error {
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0660 {
		return fmt.Errorf("invalid updater socket type or permissions")
	}
	if err := validateManagedOwner(info); err != nil {
		return fmt.Errorf("invalid updater socket owner: %w", err)
	}
	return nil
}
