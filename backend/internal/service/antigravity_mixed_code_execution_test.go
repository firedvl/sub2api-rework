//go:build unit

package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAntigravityMixedCodeExecutionDetection(t *testing.T) {
	for _, body := range []string{
		`{"tools":[{"functionDeclarations":[{"name":"f"}]},{"codeExecution":{}}]}`,
		`{"request":{"tools":[{"functionDeclarations":[{"name":"f"}],"codeExecution":{}}]}}`,
	} {
		require.True(t, antigravityV1InternalUsesMixedTools([]byte(body)))
	}
	for _, body := range []string{
		`{"tools":[{"codeExecution":{}}]}`,
		`{"tools":[{"functionDeclarations":[],"codeExecution":{}}]}`,
		`{"tools":[{"functionDeclarations":[{"name":"f"}]}]}`,
	} {
		require.False(t, antigravityV1InternalUsesMixedTools([]byte(body)))
	}
}

func TestAntigravityMixedCodeExecutionRejectedBeforeOAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"messages", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			body := []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}],"max_tokens":16,"tools":[{"type":"code_execution","name":"code_execution"},{"name":"get_weather","input_schema":{"type":"object"}}]}`)
			if protocol == "gemini" {
				body = []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"tools":[{"functionDeclarations":[{"name":"get_weather"}]},{"codeExecution":{}}],"toolConfig":{"includeServerSideToolInvocations":true}}`)
			}
			recorder := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(recorder)
			requestContext.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			upstream := &queuedHTTPUpstreamStub{}
			service := &AntigravityGatewayService{
				settingService: NewSettingService(&antigravitySettingRepoStub{}, &config.Config{}),
				httpUpstream:   upstream,
			}
			account := &Account{ID: 104, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Status: StatusActive,
				Credentials: map[string]any{"project_id": "project-104"}}
			var result *ForwardResult
			var err error
			if protocol == "gemini" {
				result, err = service.ForwardGemini(context.Background(), requestContext, account, "gemini-2.5-flash", "generateContent", false, body, false)
			} else {
				result, err = service.Forward(context.Background(), requestContext, account, body, false)
			}
			require.EqualError(t, err, AntigravityMixedToolsUnsupportedClientMessage)
			require.Nil(t, result)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Empty(t, upstream.requestBodies)
		})
	}
}
