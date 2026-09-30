package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_Forward_WSv2_TurnStateBoundToExecutionScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		respHeader := http.Header{}
		respHeader.Set("x-codex-turn-state", "turn-state-from-upstream")
		conn, err := upgrader.Upgrade(writer, request, respHeader)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		var payload map[string]any
		if err := conn.ReadJSON(&payload); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":    "resp_exec_scope_http",
				"model": "gpt-5.1",
				"usage": map[string]any{"input_tokens": 2, "output_tokens": 1},
			},
		}); err != nil {
			t.Errorf("write response.completed failed: %v", err)
		}
	}))
	defer wsServer.Close()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 0

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
	groupID := int64(9)
	account := &Account{
		ID:          456,
		Name:        "openai-http-exec-scope",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": wsServer.URL},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	reqBody := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	rec := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(rec)
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	requestContext.Request.Header.Set("session_id", "root-session")
	requestContext.Request.Header.Set(openAIWSTurnMetadataHeader, `{"session_id":"root-session","thread_id":"child-thread"}`)
	requestContext.Set("api_key", &APIKey{ID: 21, GroupID: &groupID})

	result, err := svc.Forward(context.Background(), requestContext, account, reqBody)
	require.NoError(t, err)
	require.NotNil(t, result)

	legacyHash := svc.GenerateSessionHash(requestContext, reqBody)
	scope, _ := resolveOpenAIWSExecutionScope(requestContext, reqBody, 21)
	require.NotEmpty(t, legacyHash)
	require.Len(t, scope, 16)
	require.NotEqual(t, legacyHash, scope)

	store := svc.getOpenAIWSStateStore()
	_, turnStateOnLegacy := store.GetSessionTurnState(groupID, legacyHash)
	require.False(t, turnStateOnLegacy, "turn state 不得绑定到按 session_id 算出的会话哈希")
	turnState, turnStateOnScope := store.GetSessionTurnState(groupID, scope)
	require.True(t, turnStateOnScope, "turn state 应绑定到执行作用域")
	require.Equal(t, "turn-state-from-upstream", turnState)
}

func TestOpenAIGatewayService_Forward_WSv2_ExecutionScopeUsesOriginalIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	completed := func(id string) []byte {
		return []byte(`{"type":"response.completed","response":{"id":"` + id + `","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`)
	}
	captureConn := &openAIWSCaptureConn{events: [][]byte{completed("resp_a"), completed("resp_b"), completed("resp_c")}}
	handshake := http.Header{}
	handshake.Set(openAIWSTurnStateHeader, "turn-state-from-upstream")
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSCaptureDialer{conn: captureConn, handshake: handshake})
	defer pool.Close()

	stateStore := NewOpenAIWSStateStore(nil)
	svc := &OpenAIGatewayService{
		cfg:                cfg,
		httpUpstream:       &httpUpstreamRecorder{},
		cache:              &stubGatewayCache{},
		openaiWSResolver:   NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:      NewCodexToolCorrector(),
		openaiWSPool:       pool,
		openaiWSStateStore: stateStore,
	}
	groupID := int64(9)
	const apiKeyID = int64(21)
	account := &Account{
		ID:          457,
		Name:        "openai-oauth-fingerprint-full",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-1"},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
			codexFingerprintModeExtraKey:      "full",
			codexFingerprintSeedExtraKey:      "11111111-1111-4111-8111-aaaaaaaaaaaa",
		},
	}

	forward := func(sessionID, body string) (*gin.Context, []byte) {
		rec := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(rec)
		requestContext.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		requestContext.Request.Header.Set("session_id", sessionID)
		requestContext.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		raw := []byte(body)
		result, err := svc.Forward(context.Background(), requestContext, account, raw)
		require.NoError(t, err)
		require.NotNil(t, result)
		return requestContext, raw
	}

	plainBody := `{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`
	cA, rawA := forward("session-a", plainBody)
	cB, rawB := forward("session-b", plainBody)

	scopeA, _ := resolveOpenAIWSExecutionScope(cA, rawA, apiKeyID)
	scopeB, _ := resolveOpenAIWSExecutionScope(cB, rawB, apiKeyID)
	require.NotEmpty(t, scopeA)
	require.NotEqual(t, scopeA, scopeB, "不同 session_id 的会话必须落在不同的作用域")
	_, boundA := stateStore.GetSessionTurnState(groupID, scopeA)
	require.True(t, boundA, "会话 A 的 turn state 应落在按原始 session_id 算出的作用域")
	_, boundB := stateStore.GetSessionTurnState(groupID, scopeB)
	require.True(t, boundB, "会话 B 的 turn state 应落在按原始 session_id 算出的作用域")

	injected := resolveCodexFingerprintIDs(account, "", codexFingerprintFull)
	require.NotNil(t, injected)
	injectedScope, _ := deriveOpenAISessionHashes(fmt.Sprintf("openai_ws_exec:%d|thread=%s", apiKeyID, injected.threadID))
	_, boundToInjected := stateStore.GetSessionTurnState(groupID, injectedScope)
	require.False(t, boundToInjected, "指纹收敛注入的固定 thread_id 不得成为状态键")

	threadBody := `{"model":"gpt-5.1","stream":false,"client_metadata":{"thread_id":"child-thread"},"input":[{"type":"input_text","text":"hello"}]}`
	cC, rawC := forward("session-requestContext", threadBody)
	scopeC, threadC := resolveOpenAIWSExecutionScope(cC, rawC, apiKeyID)
	require.Equal(t, "child-thread", threadC)
	_, boundC := stateStore.GetSessionTurnState(groupID, scopeC)
	require.True(t, boundC, "客户端自带线程标识时，键必须与按原始报文算出的一致，不受账号 namespace 改写影响")
}
