package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicCancellationUpstream struct {
	do       func(*http.Request) (*http.Response, error)
	attempts int
}

func (u *anthropicCancellationUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.attempts++
	return u.do(req)
}

func (u *anthropicCancellationUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

type anthropicCancellationBody struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func (b *anthropicCancellationBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if n > 0 {
		return n, nil
	}
	if err == io.EOF {
		<-b.closed
		return 0, io.ErrClosedPipe
	}
	return n, err
}

func (b *anthropicCancellationBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

type anthropicCancelOnWrite struct {
	gin.ResponseWriter
	cancel context.CancelFunc
}

func (w *anthropicCancelOnWrite) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.cancel()
	return n, err
}

func TestAnthropicAPIOnlyCancellationBeforeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(WithAnthropicAPIOnly(t.Context()))
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/anthropic/v1/messages", nil).WithContext(ctx)
	upstream := &anthropicCancellationUpstream{do: func(req *http.Request) (*http.Response, error) {
		cancel()
		if req.Context().Err() == nil {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, req.Context().Err()
	}}
	svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, accountRepo: &anthropicAPIAdmissionRepo{account: newAnthropicAPIKeyAccountForTest()}}
	result, err := svc.forwardAnthropicAPIKeyPassthrough(ctx, c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"x","stream":true}`), "x", "x", true, time.Now())
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Equal(t, 1, upstream.attempts)
	var failover *UpstreamFailoverError
	require.NotErrorAs(t, err, &failover)
}

func TestAnthropicAPIOnlyCancellationAfterUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, trigger := range []string{"context", "write"} {
		t.Run(trigger, func(t *testing.T) {
			ctx, cancel := context.WithCancel(WithAnthropicAPIOnly(t.Context()))
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/anthropic/v1/messages", nil).WithContext(ctx)
			if trigger == "context" {
				c.Writer = &anthropicCancelOnWrite{ResponseWriter: c.Writer, cancel: cancel}
			} else {
				c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
			}
			body := &anthropicCancellationBody{
				Reader: strings.NewReader("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11,\"cache_read_input_tokens\":7}}}\n"),
				closed: make(chan struct{}),
			}
			t.Cleanup(func() { _ = body.Close() })
			upstream := &anthropicCancellationUpstream{do: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body}, nil
			}}
			svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, accountRepo: &anthropicAPIAdmissionRepo{account: newAnthropicAPIKeyAccountForTest()}}
			var result *ForwardResult
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				result, err = svc.forwardAnthropicAPIKeyPassthrough(ctx, c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"x","stream":true}`), "x", "x", true, time.Now())
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				_ = body.Close()
				<-done
				t.Error("API-only forward kept draining upstream after client cancellation")
			}
			require.Error(t, err)
			if trigger == "context" {
				require.ErrorIs(t, err, context.Canceled)
			}
			require.NotNil(t, result)
			require.True(t, result.ClientDisconnect)
			require.Equal(t, boolPtr(false), result.UsageObservationComplete)
			require.Equal(t, 11, result.Usage.InputTokens)
			require.Equal(t, 7, result.Usage.CacheReadInputTokens)
			require.Equal(t, 1, upstream.attempts)
			select {
			case <-body.closed:
			default:
				t.Error("upstream body was not closed")
			}
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
		})
	}
}

func TestAnthropicAPIOnlyCanceledResponseDoesNotRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(WithAnthropicAPIOnly(t.Context()))
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/anthropic/v1/messages", nil).WithContext(ctx)
	body := &anthropicCancellationBody{Reader: strings.NewReader(""), closed: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	upstream := &anthropicCancellationUpstream{do: func(*http.Request) (*http.Response, error) {
		cancel()
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: body}, nil
	}}
	svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, accountRepo: &anthropicAPIAdmissionRepo{account: newAnthropicAPIKeyAccountForTest()}}
	done := make(chan struct{})
	var result *ForwardResult
	var err error
	go func() {
		defer close(done)
		result, err = svc.forwardAnthropicAPIKeyPassthrough(ctx, c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"x","stream":true}`), "x", "x", true, time.Now())
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = body.Close()
		<-done
		t.Error("canceled request tried to consume the error response before returning")
	}
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Equal(t, 1, upstream.attempts)
	select {
	case <-body.closed:
	default:
		t.Error("canceled response body was not closed")
	}
}

func TestAnthropicAPIOnlyStreamUpstreamContext(t *testing.T) {
	for _, apiOnly := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		if apiOnly {
			ctx = WithAnthropicAPIOnly(ctx)
		}
		upstream, release := detachStreamUpstreamContext(ctx, true)
		cancel()
		release()
		if apiOnly {
			require.ErrorIs(t, upstream.Err(), context.Canceled)
		} else {
			require.NoError(t, upstream.Err())
		}
	}
}

func TestAnthropicLegacyWriteFailureStillDrainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(
		"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11}}}\n\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":5}}\n\ndata: [DONE]\n\n"))}
	result, err := (&GatewayService{}).handleStreamingResponseAnthropicAPIKeyPassthrough(t.Context(), resp, c, &Account{ID: 1}, time.Now(), "x")
	require.NoError(t, err)
	require.True(t, result.clientDisconnect)
	require.Equal(t, 11, result.usage.InputTokens)
	require.Equal(t, 5, result.usage.OutputTokens)
}
