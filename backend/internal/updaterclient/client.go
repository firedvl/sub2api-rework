package updaterclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/updatecontract"
)

const (
	DefaultSocketPath = "/run/sub2api-rework-updater/updater.sock"
	maxResponseBytes  = 64 * 1024
)

var ErrUnavailable = errors.New("updater service unavailable")

type Client struct {
	socketPath string
	httpClient *http.Client
	configErr  error
}

func New() *Client {
	socketPath := strings.TrimSpace(os.Getenv("SUB2API_UPDATER_SOCKET"))
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return NewForSocket(socketPath)
}

func NewForSocket(socketPath string) *Client {
	client := &Client{socketPath: socketPath}
	if !filepath.IsAbs(socketPath) || filepath.Clean(socketPath) != socketPath {
		client.configErr = fmt.Errorf("updater socket path must be clean and absolute")
	}
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	client.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DisableCompression: true,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", client.socketPath)
			},
		},
	}
	return client
}

func (c *Client) Status(ctx context.Context) (*updatecontract.UpdaterStatus, error) {
	var status updatecontract.UpdaterStatus
	if err := c.do(ctx, http.MethodGet, "/v1/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) Prepare(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return c.start(ctx, "/v1/prepare", request)
}

func (c *Client) Install(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return c.start(ctx, "/v1/install", request)
}

func (c *Client) Rollback(ctx context.Context, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	return c.start(ctx, "/v1/rollback", request)
}

func (c *Client) start(ctx context.Context, path string, request updatecontract.OperationRequest) (*updatecontract.OperationAccepted, error) {
	var accepted updatecontract.OperationAccepted
	if err := c.do(ctx, http.MethodPost, path, request, &accepted); err != nil {
		return nil, err
	}
	return &accepted, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, target any) error {
	if c.configErr != nil {
		return fmt.Errorf("%w: invalid client configuration", ErrUnavailable)
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://updater"+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w", ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("updater request rejected with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return fmt.Errorf("invalid updater response")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid updater response")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid updater response")
	}
	return nil
}
