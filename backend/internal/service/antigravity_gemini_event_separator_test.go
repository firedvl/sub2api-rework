//go:build unit

package service

import (
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleGeminiStreamingResponse_EventSeparatorIsExactlyOneBlankLine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newAntigravityTestService(&config.Config{
		Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize},
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{}}

	first := `{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3}}`
	second := `{"candidates":[{"content":{"role":"model","parts":[{"thoughtSignature":"sig","text":""}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3,"thoughtsTokenCount":5}}`

	go func() {
		defer func() { _ = pw.Close() }()
		fmt.Fprintf(pw, "data: %s\n\n", first)
		fmt.Fprintf(pw, "data: %s\r\n\r\n", second)
		fmt.Fprint(pw, "data: [DONE]\n\n")
	}()

	result, err := svc.handleGeminiStreamingResponse(c, resp, time.Now())
	_ = pr.Close()

	require.NoError(t, err)
	require.NotNil(t, result)

	body := rec.Body.String()
	require.Equal(t, "data: "+first+"\n\ndata: "+second+"\n\ndata: [DONE]\n\n", body)
	require.NotContains(t, body, "\n\n\n", "events must be separated by exactly one blank line")

	for _, token := range strings.Split(strings.TrimSuffix(body, "\n\n"), "\n\n") {
		prefix, _, _ := strings.Cut(token, ":")
		require.Equal(t, "data", prefix, "token %q would be rejected by a \\n\\n-delimited SSE parser", token)
	}
}
