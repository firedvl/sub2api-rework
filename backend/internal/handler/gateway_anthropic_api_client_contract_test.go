//go:build unit

package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// This transport cannot send fixture keys to any host except the owned loopback provider.
type anthropicClientContractUpstream struct {
	host   string
	client *http.Client
}

func (u *anthropicClientContractUpstream) Do(req *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
	if req.URL.Scheme != "http" || req.URL.Host != u.host || proxy != "" {
		return nil, fmt.Errorf("contract forbids non-fixture upstream %s", req.URL.Redacted())
	}
	return u.client.Do(req)
}

func (u *anthropicClientContractUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

type anthropicClientContractAffinity struct {
	service.GatewayCache
	mu       sync.Mutex
	accounts map[string]int64
	hits     int
}

func (s *anthropicClientContractAffinity) GetSessionAccountID(_ context.Context, group int64, hash string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.accounts[fmt.Sprintf("%d/%s", group, hash)]
	if id == 72 {
		s.hits++
	}
	return id, nil
}
func (s *anthropicClientContractAffinity) SetSessionAccountID(_ context.Context, group int64, hash string, id int64, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[fmt.Sprintf("%d/%s", group, hash)] = id
	return nil
}
func (s *anthropicClientContractAffinity) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}
func (s *anthropicClientContractAffinity) DeleteSessionAccountID(_ context.Context, group int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accounts, fmt.Sprintf("%d/%s", group, hash))
	return nil
}

type anthropicClientContractRequest struct {
	path   string
	header http.Header
	body   []byte
}

func anthropicClientContractSSE(w http.ResponseWriter, model string, block map[string]any, stop string) {
	start := map[string]any{"type": "text", "text": ""}
	delta := map[string]any{"type": "text_delta", "text": block["text"]}
	if block["type"] == "tool_use" {
		start = map[string]any{"type": "tool_use", "id": block["id"], "name": block["name"], "input": map[string]any{}}
		input, _ := json.Marshal(block["input"])
		delta = map[string]any{"type": "input_json_delta", "partial_json": string(input)}
	}
	events := []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "msg_synthetic", "type": "message", "role": "assistant", "model": model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]any{"input_tokens": 8, "output_tokens": 0, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}},
		{"type": "content_block_start", "index": 0, "content_block": start},
		{"type": "content_block_delta", "index": 0, "delta": delta},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 3}},
		{"type": "message_stop"},
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range events {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

func TestGatewayAnthropicAPIOnlyOfficialClientContract(t *testing.T) {
	binary := os.Getenv("CLAUDE_GATEWAY_CONTRACT_BINARY")
	if binary == "" {
		t.Skip("set CLAUDE_GATEWAY_CONTRACT_BINARY to the qualified official Claude Code binary")
	}
	require.True(t, filepath.IsAbs(binary), "the opt-in binary must be absolute")
	root := t.TempDir()
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root,
		"CLAUDE_CONFIG_DIR=" + filepath.Join(root, "claude"), "ANTHROPIC_CONFIG_DIR=" + filepath.Join(root, "anthropic"),
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"}
	versionCtx, versionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer versionCancel()
	version := exec.CommandContext(versionCtx, binary, "--version")
	version.Env, version.Dir = env, root
	output, err := version.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Equal(t, "2.1.300 (Claude Code)", strings.TrimSpace(string(output)), "unknown client versions must be qualified explicitly")

	gin.SetMode(gin.TestMode)
	fixture := filepath.Join(root, "fixture.txt")
	require.NoError(t, os.WriteFile(fixture, []byte(strings.Repeat("DISPOSABLE_CONTRACT_DATA\n", 128)), 0600))
	var mu sync.Mutex
	var requests []anthropicClientContractRequest
	turn := 0
	upstreamCanceled := make(chan struct{})
	gatewayCanceled := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, readErr.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, anthropicClientContractRequest{r.URL.Path, r.Header.Clone(), body})
		mu.Unlock()
		if r.URL.Path == "/v1/messages/count_tokens" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"input_tokens":8}`)
			return
		}
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "unexpected fixture path", http.StatusNotFound)
			return
		}
		if gjson.GetBytes(body, "contract_cancel").Bool() {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_partial\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\n")
			io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"PARTIAL_FIXTURE_OUTPUT\"}}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(upstreamCanceled)
			return
		}
		if r.Header.Get("x-api-key") == "fixture-A" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"synthetic account A cooldown"}}`)
			return
		}
		mu.Lock()
		turn++
		currentTurn := turn
		mu.Unlock()
		block := map[string]any{"type": "text", "text": "SYNTHETIC_GATEWAY_COMPLETE"}
		stop := "end_turn"
		if currentTurn <= 2 {
			block = map[string]any{"type": "tool_use", "id": fmt.Sprintf("read_%d", currentTurn), "name": "Read", "input": map[string]any{"file_path": fixture}}
			stop = "tool_use"
		}
		anthropicClientContractSSE(w, gjson.GetBytes(body, "model").String(), block, stop)
	}))
	defer provider.Close()
	providerURL, err := url.Parse(provider.URL)
	require.NoError(t, err)
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	upstream := &anthropicClientContractUpstream{host: providerURL.Host, client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	groupID := int64(70)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	accounts := []*service.Account{}
	accountValues := []service.Account{}
	for i, id := range []int64{71, 72} {
		account := service.Account{ID: id, Name: fmt.Sprintf("fixture-%d", id), Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
			Priority: i, Concurrency: 1, Schedulable: true, Status: service.StatusActive,
			Credentials:   map[string]any{"api_key": fmt.Sprintf("fixture-%c", 'A'+i), "base_url": provider.URL},
			AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}}}
		accountValues = append(accountValues, account)
		accounts = append(accounts, &account)
	}
	repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: accountValues}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 16)}
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{
		service.SettingKeyRateLimit429CooldownSettings: `{"enabled":true,"cooldown_seconds":120}`,
	}}, cfg)
	rateLimits := service.NewRateLimitService(repo, usageRepo, cfg, nil, nil)
	rateLimits.SetSettingService(settings)
	affinity := &anthropicClientContractAffinity{accounts: map[string]int64{}}
	concurrency := service.NewConcurrencyService(&fakeConcurrencyCache{})
	gateway := service.NewGatewayService(repo, &fakeGroupRepo{group: group}, usageRepo, nil, nil, nil, nil, affinity, cfg,
		service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil),
		concurrency, service.NewBillingService(cfg, nil), rateLimits, nil, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	defer billingCache.Stop()
	h := &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1,
		settingService:    settings,
		concurrencyHelper: NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0), usageRecordWorkerPool: newUsageRecordTestPool(t)}
	key := &service.APIKey{ID: 75, UserID: 76, GroupID: &groupID, Group: group, Status: service.StatusActive,
		User: &service.User{ID: 76, Concurrency: 10, Balance: 100}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("x-api-key") != "fixture-downstream-key" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx := service.WithAnthropicAPIOnly(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Request = c.Request.WithContext(ctx)
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
		c.Next()
	})
	router.POST("/anthropic-api/v1/messages", func(c *gin.Context) {
		h.Messages(c)
		if c.GetHeader("X-Contract-Cancel") == "1" {
			close(gatewayCanceled)
		}
	})
	router.POST("/anthropic-api/v1/messages/count_tokens", h.CountTokens)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "--bare", "--restricted", "-p", "Read the disposable fixture as requested.",
		"--model", "claude-sonnet-4-6", "--tools", "Read", "--allowedTools", "Read", "--permission-mode", "dontAsk",
		"--strict-mcp-config", "--setting-sources", "", "--disable-slash-commands", "--system-prompt", "Offline synthetic contract. Only read the supplied fixture.",
		"--max-turns", "4", "--no-session-persistence", "--output-format", "json")
	command.Env = append(env, "ANTHROPIC_API_KEY=fixture-downstream-key", "ANTHROPIC_BASE_URL="+server.URL+"/anthropic-api")
	command.Dir = root
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "SYNTHETIC_GATEWAY_COMPLETE")
	for i := 0; i < 3; i++ {
		select {
		case usage := <-usageRepo.created:
			require.NotNil(t, usage.UsageObservationComplete)
			require.True(t, *usage.UsageObservationComplete)
			require.Equal(t, int64(72), usage.AccountID)
			require.Equal(t, key.ID, usage.APIKeyID)
			require.Equal(t, key.UserID, usage.UserID)
			require.Equal(t, groupID, *usage.GroupID)
			require.EqualValues(t, 8, usage.InputTokens)
			require.EqualValues(t, 3, usage.OutputTokens)
		case <-time.After(5 * time.Second):
			t.Fatal("successful tool cycle did not record usage")
		}
	}
	// The CLI need not request token counts for a small prompt; probe the same wire route explicitly.
	mu.Lock()
	metadata := json.RawMessage(gjson.GetBytes(requests[1].body, "metadata").Raw)
	mu.Unlock()
	countRequest, err := json.Marshal(map[string]any{"model": "claude-sonnet-4-6", "messages": []map[string]string{{"role": "user", "content": "fixture"}}, "metadata": metadata})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/anthropic-api/v1/messages/count_tokens", strings.NewReader(string(countRequest)))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "fixture-downstream-key")
	req.Header.Set("Cookie", "fixture-downstream-cookie=never-forward")
	req.Header.Set("Authorization", "Bearer fixture-downstream-token")
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Transport: transport}).Do(req)
	require.NoError(t, err)
	countBody, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, string(countBody))
	require.JSONEq(t, `{"input_tokens":8}`, string(countBody))
	// A new session must also bypass A's stale scheduler snapshot after its authoritative cooldown.
	freshCount, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/anthropic-api/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"fresh synthetic session"}]}`))
	require.NoError(t, err)
	freshCount.Header.Set("x-api-key", "fixture-downstream-key")
	freshCount.Header.Set("Content-Type", "application/json")
	response, err = (&http.Client{Transport: transport}).Do(freshCount)
	require.NoError(t, err)
	countBody, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, string(countBody))
	require.JSONEq(t, `{"input_tokens":8}`, string(countBody))
	// Cancel after receiving partial SSE bytes over real sockets, before any final usage exists.
	cancelBody, err := json.Marshal(map[string]any{"model": "claude-sonnet-4-6", "max_tokens": 16, "stream": true,
		"messages": []map[string]string{{"role": "user", "content": "cancel fixture"}}, "metadata": metadata, "contract_cancel": true})
	require.NoError(t, err)
	cancelCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	cancelRequest, err := http.NewRequestWithContext(cancelCtx, http.MethodPost, server.URL+"/anthropic-api/v1/messages", strings.NewReader(string(cancelBody)))
	require.NoError(t, err)
	cancelRequest.Header.Set("x-api-key", "fixture-downstream-key")
	cancelRequest.Header.Set("Content-Type", "application/json")
	cancelRequest.Header.Set("X-Contract-Cancel", "1")
	response, err = (&http.Client{Transport: transport}).Do(cancelRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	partial := ""
	reader := bufio.NewReader(response.Body)
	for lines := 0; lines < 8 && !strings.Contains(partial, "PARTIAL_FIXTURE_OUTPUT"); lines++ {
		line, readErr := reader.ReadString('\n')
		require.NoError(t, readErr)
		partial += line
	}
	require.Contains(t, partial, "PARTIAL_FIXTURE_OUTPUT")
	require.NotContains(t, partial, "message_stop")
	cancelStream()
	response.Body.Close()
	for _, signal := range []<-chan struct{}{upstreamCanceled, gatewayCanceled} {
		select {
		case <-signal:
		case <-time.After(5 * time.Second):
			t.Fatal("client cancellation did not close the gateway and upstream stream")
		}
	}
	select {
	case usage := <-usageRepo.created:
		require.NotNil(t, usage.UsageObservationComplete)
		require.False(t, *usage.UsageObservationComplete)
		require.Equal(t, int64(72), usage.AccountID)
		require.EqualValues(t, 8, usage.InputTokens)
		require.EqualValues(t, 0, usage.OutputTokens)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled observed response did not retain incomplete usage")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 7, "one bounded failover, three successful cycles, two token counts, and one canceled stream without replay")
	for index, request := range requests {
		wantKey := "fixture-B"
		if index == 0 {
			wantKey = "fixture-A"
		}
		require.Equal(t, wantKey, request.header.Get("x-api-key"))
		require.Empty(t, request.header.Get("Authorization"))
		require.Empty(t, request.header.Get("Cookie"))
		require.Equal(t, "claude-sonnet-4-6", gjson.GetBytes(request.body, "model").String())
	}
	require.JSONEq(t, string(requests[0].body), string(requests[1].body), "failover must preserve the native request")
	for index, request := range requests[1:4] {
		require.True(t, gjson.GetBytes(request.body, "stream").Bool())
		require.Equal(t, "Read", gjson.GetBytes(request.body, "tools.0.name").String())
		require.Equal(t, "adaptive", gjson.GetBytes(request.body, "thinking.type").String())
		require.True(t, gjson.GetBytes(request.body, "context_management.edits").Exists())
		require.Contains(t, string(request.body), `"cache_control"`)
		for n := 1; n <= index; n++ {
			require.Contains(t, string(request.body), fmt.Sprintf(`"tool_use_id":"read_%d"`, n))
		}
		if index > 0 {
			require.Contains(t, string(request.body), "DISPOSABLE_CONTRACT_DATA")
		}
	}
	affinity.mu.Lock()
	defer affinity.mu.Unlock()
	require.Positive(t, affinity.hits, "subsequent client cycles must resolve the selected B affinity")
	t.Log("Official Claude Code 2.1.300 completed two real Read tools through API-only Go routing, A-to-B failover, B affinity, usage attribution, and native fields; raw HTTP probes passed count_tokens and partial-stream cancellation without replay")
}
