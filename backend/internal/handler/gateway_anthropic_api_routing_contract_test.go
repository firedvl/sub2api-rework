//go:build unit

package handler

import (
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
	"syscall"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Keep repository observations authoritative while the scheduler snapshot remains stale.
type anthropicRoutingContractRepo struct {
	*openAIWSFailoverHandlerAccountRepoStub
	mu sync.Mutex
}

func (r *anthropicRoutingContractRepo) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.openAIWSFailoverHandlerAccountRepoStub.GetByID(ctx, id)
}
func (r *anthropicRoutingContractRepo) SetRateLimited(ctx context.Context, id int64, until time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.openAIWSFailoverHandlerAccountRepoStub.SetRateLimited(ctx, id, until)
}
func (r *anthropicRoutingContractRepo) update(id int64, change func(*service.Account)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			change(&r.accounts[i])
			return
		}
	}
}

type anthropicRoutingContractSlots struct {
	fakeConcurrencyCache
	mu     sync.Mutex
	active map[int64]int
	peak   map[int64]int
}

func (s *anthropicRoutingContractSlots) AcquireAccountSlot(_ context.Context, id int64, max int, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[id] >= max {
		return false, nil
	}
	s.active[id]++
	if s.active[id] > s.peak[id] {
		s.peak[id] = s.active[id]
	}
	return true, nil
}
func (s *anthropicRoutingContractSlots) ReleaseAccountSlot(_ context.Context, id int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[id]--
	return nil
}
func (s *anthropicRoutingContractSlots) GetAccountsLoadBatch(_ context.Context, accounts []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := map[int64]*service.AccountLoadInfo{}
	for _, a := range accounts {
		result[a.ID] = &service.AccountLoadInfo{AccountID: a.ID, CurrentConcurrency: s.active[a.ID], LoadRate: s.active[a.ID] * 100 / a.MaxConcurrency}
	}
	return result, nil
}

type anthropicRoutingContract struct {
	server    *httptest.Server
	repo      *anthropicRoutingContractRepo
	slots     *anthropicRoutingContractSlots
	client    *http.Client
	snapshots []*service.Account
}

func newAnthropicRoutingContract(t *testing.T, serve http.HandlerFunc) *anthropicRoutingContract {
	t.Helper()
	gin.SetMode(gin.TestMode)
	provider := httptest.NewServer(serve)
	t.Cleanup(provider.Close)
	parsed, err := url.Parse(provider.URL)
	require.NoError(t, err)
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	groupID := int64(70)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	values := []service.Account{}
	snapshots := []*service.Account{}
	for i, id := range []int64{71, 72} {
		a := service.Account{ID: id, Name: fmt.Sprintf("synthetic-%d", id), Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Priority: 0, Concurrency: 1, Schedulable: true, Status: service.StatusActive,
			Credentials: map[string]any{"api_key": fmt.Sprintf("fixture-%c", 'A'+i), "base_url": provider.URL}, AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}}}
		values = append(values, a)
		snapshots = append(snapshots, &a)
	}
	repo := &anthropicRoutingContractRepo{openAIWSFailoverHandlerAccountRepoStub: &openAIWSFailoverHandlerAccountRepoStub{accounts: values}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	usage := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 128)}
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{service.SettingKeyRateLimit429CooldownSettings: `{"enabled":true,"cooldown_seconds":120}`}}, cfg)
	limits := service.NewRateLimitService(repo, usage, cfg, nil, nil)
	limits.SetSettingService(settings)
	affinity := &anthropicClientContractAffinity{accounts: map[string]int64{}}
	slots := &anthropicRoutingContractSlots{active: map[int64]int{}, peak: map[int64]int{}}
	concurrency := service.NewConcurrencyService(slots)
	gateway := service.NewGatewayService(repo, &fakeGroupRepo{group: group}, usage, nil, nil, nil, nil, affinity, cfg,
		service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: snapshots}, nil, nil, nil, nil), concurrency, service.NewBillingService(cfg, nil), limits, nil, nil,
		&anthropicClientContractUpstream{host: parsed.Host, client: client}, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := &GatewayHandler{gatewayService: gateway, billingCacheService: billing, cfg: cfg, maxAccountSwitches: 1, settingService: settings, concurrencyHelper: NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0), usageRecordWorkerPool: newUsageRecordTestPool(t)}
	key := &service.APIKey{ID: 75, UserID: 76, GroupID: &groupID, Group: group, Status: service.StatusActive, User: &service.User{ID: 76, Concurrency: 10, Balance: 100}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("x-api-key") != "fixture-downstream-key" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Request = c.Request.WithContext(service.WithAnthropicAPIOnly(context.WithValue(c.Request.Context(), ctxkey.Group, group)))
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
		c.Next()
	})
	router.POST("/anthropic-api/v1/messages", h.Messages)
	router.POST("/anthropic-api/v1/messages/count_tokens", h.CountTokens)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &anthropicRoutingContract{server: server, repo: repo, slots: slots, client: client, snapshots: snapshots}
}
func (f *anthropicRoutingContract) request(t *testing.T, session, model, prompt string, stream bool) (int, string) {
	t.Helper()
	metadata, _ := json.Marshal(map[string]string{"device_id": strings.Repeat("a", 64), "account_uuid": "", "session_id": session})
	body, err := json.Marshal(map[string]any{"model": model, "max_tokens": 16, "stream": stream, "messages": []map[string]string{{"role": "user", "content": prompt}}, "metadata": map[string]string{"user_id": string(metadata)}})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, f.server.URL+"/anthropic-api/v1/messages", strings.NewReader(string(body)))
	require.NoError(t, err)
	request.Header.Set("x-api-key", "fixture-downstream-key")
	request.Header.Set("Content-Type", "application/json")
	response, err := f.client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(data)
}
func anthropicRoutingContractSuccess(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	model := gjson.GetBytes(body, "model").String()
	if gjson.GetBytes(body, "stream").Bool() {
		anthropicClientContractSSE(w, model, map[string]any{"type": "text", "text": "SYNTHETIC_OK"}, "end_turn")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":"msg_fixture","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":3}}`, model, r.Header.Get("x-api-key"))
}

func TestGatewayAnthropicAPIOnlyRoutingContracts(t *testing.T) {
	const session = "123e4567-e89b-12d3-a456-426614174000"
	t.Run("concurrent independent sessions use both account slots", func(t *testing.T) {
		arrived := make(chan string, 2)
		release := make(chan struct{})
		defer close(release)
		f := newAnthropicRoutingContract(t, func(w http.ResponseWriter, r *http.Request) {
			arrived <- r.Header.Get("x-api-key")
			<-release
			anthropicRoutingContractSuccess(w, r)
		})
		completed := make(chan struct{}, 2)
		for i := 0; i < 2; i++ {
			go func(i int) {
				defer func() { completed <- struct{}{} }()
				code, _ := f.request(t, fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", i), "claude-sonnet-4-6", "parallel", false)
				require.Equal(t, 200, code)
			}(i)
		}
		selected := map[string]bool{}
		for i := 0; i < 2; i++ {
			select {
			case account := <-arrived:
				selected[account] = true
			case <-time.After(5 * time.Second):
				t.Fatal("independent request did not acquire the other free account")
			}
		}
		require.Len(t, selected, 2)
		release <- struct{}{}
		release <- struct{}{}
		for i := 0; i < 2; i++ {
			<-completed
		}
		f.slots.mu.Lock()
		defer f.slots.mu.Unlock()
		require.Equal(t, map[int64]int{71: 1, 72: 1}, f.slots.peak)
		require.Equal(t, map[int64]int{71: 0, 72: 0}, f.slots.active)
	})
	t.Run("same session survives turns but disabled account invalidates affinity", func(t *testing.T) {
		f := newAnthropicRoutingContract(t, anthropicRoutingContractSuccess)
		code, first := f.request(t, session, "claude-sonnet-4-6", "first", false)
		require.Equal(t, 200, code)
		_, second := f.request(t, session, "claude-sonnet-4-6", "next", false)
		require.Equal(t, gjson.Get(first, "content.0.text").String(), gjson.Get(second, "content.0.text").String())
		selected := int64(71)
		if strings.Contains(first, "fixture-B") {
			selected = 72
		}
		f.repo.update(selected, func(a *service.Account) { a.Schedulable = false })
		code, third := f.request(t, session, "claude-sonnet-4-6", "after disable", false)
		require.Equal(t, 200, code)
		require.NotEqual(t, gjson.Get(first, "content.0.text").String(), gjson.Get(third, "content.0.text").String())
	})
	t.Run("model change invalidates incompatible affinity", func(t *testing.T) {
		f := newAnthropicRoutingContract(t, anthropicRoutingContractSuccess)
		f.repo.update(71, func(a *service.Account) {
			a.Credentials = map[string]any{"api_key": "fixture-A", "base_url": a.Credentials["base_url"], "model_mapping": map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"}}
		})
		f.repo.update(72, func(a *service.Account) {
			a.Credentials = map[string]any{"api_key": "fixture-B", "base_url": a.Credentials["base_url"], "model_mapping": map[string]any{"claude-opus-4-6": "claude-opus-4-6"}}
		})
		for i := range f.snapshots {
			account, err := f.repo.GetByID(context.Background(), f.snapshots[i].ID)
			require.NoError(t, err)
			f.snapshots[i] = account
		}
		code, body := f.request(t, session, "claude-sonnet-4-6", "sonnet", false)
		require.Equal(t, 200, code)
		require.Contains(t, body, "fixture-A")
		code, body = f.request(t, session, "claude-opus-4-6", "opus", false)
		require.Equal(t, 200, code)
		require.Contains(t, body, "fixture-B")
	})
	t.Run("authoritative cooldown expires without replacing stale snapshot", func(t *testing.T) {
		f := newAnthropicRoutingContract(t, anthropicRoutingContractSuccess)
		f.snapshots[0].Priority = -1
		future := time.Now().Add(time.Hour)
		f.repo.update(71, func(a *service.Account) { a.Priority = -1; a.RateLimitResetAt = &future })
		_, body := f.request(t, session, "claude-sonnet-4-6", "cooldown", false)
		require.Contains(t, body, "fixture-B")
		past := time.Now().Add(-time.Second)
		f.repo.update(71, func(a *service.Account) { a.RateLimitResetAt = &past })
		code, body := f.request(t, "123e4567-e89b-12d3-a456-426614174001", "claude-sonnet-4-6", "recovered", false)
		require.Equal(t, 200, code)
		require.Contains(t, body, "fixture-A")
	})

	t.Run("temporary 503 fails over once preserving native request", func(t *testing.T) {
		var mu sync.Mutex
		var requests []anthropicClientContractRequest
		f := newAnthropicRoutingContract(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			requests = append(requests, anthropicClientContractRequest{r.URL.Path, r.Header.Clone(), body})
			mu.Unlock()
			if r.Header.Get("x-api-key") == "fixture-A" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				io.WriteString(w, `{"type":"error","error":{"type":"overloaded_error","message":"synthetic temporary outage"}}`)
				return
			}
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			anthropicRoutingContractSuccess(w, r)
		})
		f.snapshots[0].Priority = -1
		code, body := f.request(t, session, "claude-sonnet-4-6", "temporary failover", false)
		require.Equal(t, 200, code)
		require.Contains(t, body, "fixture-B")
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, requests, 2)
		require.Equal(t, "fixture-A", requests[0].header.Get("x-api-key"))
		require.Equal(t, "fixture-B", requests[1].header.Get("x-api-key"))
		require.JSONEq(t, string(requests[0].body), string(requests[1].body))
	})
	for _, scenario := range []string{"permanent invalid request", "stream error after output"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			f := newAnthropicRoutingContract(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls++
				mu.Unlock()
				if scenario == "permanent invalid request" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(400)
					io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"synthetic invalid input"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_partial\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\n")
				io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"PARTIAL_OUTPUT\"}}\n\n")
				w.(http.Flusher).Flush()
				io.WriteString(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"synthetic interrupted stream\"}}\n\n")
			})
			code, body := f.request(t, session, "claude-sonnet-4-6", "failure", scenario != "permanent invalid request")
			if scenario == "permanent invalid request" {
				require.Equal(t, 400, code)
			} else {
				require.Equal(t, 200, code)
				require.Contains(t, body, "PARTIAL_OUTPUT")
				require.Contains(t, body, "overloaded_error")
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, 1, calls, "no other account may receive replay")
		})
	}
}

func TestGatewayAnthropicAPIOnlyOfficialClientEditResumeContract(t *testing.T) {
	binary := os.Getenv("CLAUDE_GATEWAY_CONTRACT_BINARY")
	if binary == "" {
		t.Skip("set CLAUDE_GATEWAY_CONTRACT_BINARY to the qualified official Claude Code binary")
	}
	require.True(t, filepath.IsAbs(binary))
	root := t.TempDir()
	fixture := filepath.Join(root, "edit-fixture.txt")
	require.NoError(t, os.WriteFile(fixture, []byte("DISPOSABLE_BEFORE\n"), 0600))
	var mu sync.Mutex
	var requests [][]byte
	turn := 0
	f := newAnthropicRoutingContract(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.URL.Path == "/v1/messages/count_tokens" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"input_tokens":8}`)
			return
		}
		mu.Lock()
		requests = append(requests, body)
		turn++
		current := turn
		mu.Unlock()
		block := map[string]any{"type": "text", "text": "SYNTHETIC_EDIT_COMPLETE"}
		stop := "end_turn"
		if current == 1 {
			block = map[string]any{"type": "tool_use", "id": "read_edit_fixture", "name": "Read", "input": map[string]any{"file_path": fixture}}
			stop = "tool_use"
		}
		if current == 2 {
			block = map[string]any{"type": "tool_use", "id": "edit_fixture", "name": "Edit", "input": map[string]any{"file_path": fixture, "old_string": "DISPOSABLE_BEFORE", "new_string": "DISPOSABLE_AFTER"}}
			stop = "tool_use"
		}
		if current == 4 {
			block["text"] = "SYNTHETIC_RESUME_COMPLETE"
		}
		anthropicClientContractSSE(w, gjson.GetBytes(body, "model").String(), block, stop)
	})
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "CLAUDE_CONFIG_DIR=" + filepath.Join(root, "claude"), "ANTHROPIC_CONFIG_DIR=" + filepath.Join(root, "anthropic"), "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "ANTHROPIC_API_KEY=fixture-downstream-key", "ANTHROPIC_BASE_URL=" + f.server.URL + "/anthropic-api"}
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, args...)
		command.Env = env
		command.Dir = root
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return output
	}
	require.Equal(t, "2.1.300 (Claude Code)", strings.TrimSpace(string(run("--version"))))
	status := run("--bare", "auth", "status", "--json")
	require.True(t, gjson.GetBytes(status, "loggedIn").Bool(), string(status))
	require.Equal(t, "api_key", gjson.GetBytes(status, "authMethod").String(), string(status))
	common := []string{"--bare", "--restricted", "--tools", "Read,Edit", "--allowedTools", "Read,Edit", "--permission-mode", "acceptEdits", "--strict-mcp-config", "--setting-sources", "", "--disable-slash-commands", "--system-prompt", "Offline synthetic contract. Only read and edit the supplied disposable fixture.", "--max-turns", "4", "--output-format", "json"}
	first := run(append(common, "-p", "Read and edit the disposable fixture as requested.", "--model", "claude-sonnet-4-6", "--session-id", "123e4567-e89b-42d3-a456-426614174000")...)
	require.Contains(t, string(first), "SYNTHETIC_EDIT_COMPLETE")
	edited, err := os.ReadFile(fixture)
	require.NoError(t, err)
	require.Equal(t, "DISPOSABLE_AFTER\n", string(edited))
	sessionID := gjson.GetBytes(first, "session_id").String()
	require.NotEmpty(t, sessionID)
	resumed := run(append(common, "-p", "Continue the disposable conversation.", "--model", "claude-opus-4-6", "--resume", sessionID)...)
	require.Contains(t, string(resumed), "SYNTHETIC_RESUME_COMPLETE")

	// Exercise the real terminal startup and local /status command without inference.
	python, err := exec.LookPath("python3")
	require.NoError(t, err)
	script := `import errno, fcntl, os, pty, re, select, signal, struct, sys, termios, time
pid, fd = pty.fork()
if pid == 0:
 fcntl.ioctl(0,termios.TIOCSWINSZ,struct.pack("HHHH",32,120,0,0))
 os.environ["TERM"]="xterm-256color"
 os.execv(sys.argv[1], [sys.argv[1], "--bare", "--restricted", "--model", "claude-sonnet-4-6", "--tools", "", "--strict-mcp-config", "--setting-sources", ""])
output=b""
def plain():
 return re.sub(rb"\s+",b" ",re.sub(rb"\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*\x07|.)",b" ",output))
def read_until(predicate, seconds=10):
 global output
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if predicate(plain()): return True
  ready,_,_=select.select([fd],[],[],min(0.1,end-time.monotonic()))
  if ready:
   try: chunk=os.read(fd,65536)
   except OSError as exc:
    if exc.errno==errno.EIO: return False
    raise
   if not chunk: return False
   output+=chunk
 return predicate(plain())
def interrupted(signum, frame):
 raise TimeoutError("interactive contract canceled")
signal.signal(signal.SIGTERM,interrupted)
signal.signal(signal.SIGINT,interrupted)
try:
 if not read_until(lambda data: b"Choose" in data or b"Try" in data):
  raise AssertionError("interactive startup missing")
 if b"Choose" in plain():
  output=b""
  os.write(fd,b"\r")
  if not read_until(lambda data: b"custom" in data or b"Security notes" in data or b"Accessing" in data or b"Try" in data):
   raise AssertionError("interactive onboarding did not advance")
 if b"custom" in plain():
  output=b""
  os.write(fd,b"\x1b[A\r")
  if not read_until(lambda data: b"Security notes" in data or b"Accessing" in data or b"shortcuts" in data or b"Try" in data):
   raise AssertionError("interactive fixture API-key confirmation did not advance")
 if b"Security notes" in plain():
  output=b""
  os.write(fd,b"\r")
  if not read_until(lambda data: (b"Accessing" in data and b"cancel" in data) or b"shortcuts" in data or b"Try" in data):
   raise AssertionError("interactive security notes did not advance")
 if b"Accessing" in plain():
  # Qualified 2.1.300 rejects trust-dialog input for its first 150 ms.
  shown=time.monotonic()
  read_until(lambda _: time.monotonic()-shown >= 0.2, seconds=1)
  output=b""
  os.write(fd,b"\x1b[B")
  if not read_until(lambda data: b"Yes" in data and b"folder" in data):
   raise AssertionError("disposable workspace trust selection did not move")
  output=b""
  os.write(fd,b"\r")
  if not read_until(lambda data: b"API Usage Billing" in data and b"shift+tab" in data):
   raise AssertionError("interactive prompt missing")
 output=b""
 os.write(fd,b"/status\r")
 if not read_until(lambda data: b"127.0.0.1" in data and b"/anthropic-api" in data):
  raise AssertionError("interactive status did not show gateway base URL")
finally:
 try: os.kill(pid,signal.SIGKILL)
 except ProcessLookupError: pass
 os.waitpid(pid,0)
 os.close(fd)
 print(output.decode(errors="replace"))
`
	terminalCtx, terminalCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer terminalCancel()
	terminal := exec.CommandContext(terminalCtx, python, "-c", script, binary)
	terminal.Cancel = func() error { return terminal.Process.Signal(syscall.SIGTERM) }
	terminal.WaitDelay = 5 * time.Second
	terminal.Env = env
	terminal.Dir = root
	terminalOutput, err := terminal.CombinedOutput()
	require.NoError(t, err, string(terminalOutput))
	require.Contains(t, string(terminalOutput), f.server.URL+"/anthropic-api")
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 4)
	for _, body := range requests[:3] {
		require.Equal(t, "claude-sonnet-4-6", gjson.GetBytes(body, "model").String())
	}
	require.Equal(t, "claude-opus-4-6", gjson.GetBytes(requests[3], "model").String())
	require.Contains(t, string(requests[2]), `"tool_use_id":"edit_fixture"`)
	require.Contains(t, string(requests[3]), "DISPOSABLE_BEFORE")
	require.Contains(t, string(requests[3]), "SYNTHETIC_EDIT_COMPLETE")
	require.Equal(t, gjson.GetBytes(requests[0], "metadata.user_id").String(), gjson.GetBytes(requests[3], "metadata.user_id").String(), "resuming and switching model keeps the native session identity")
	t.Log("Official Claude Code 2.1.300 API-key auth status, interactive startup/status, Read/Edit, session persistence/resume, model switch and retained tool history passed against the real API-only Go handler; no real provider calls")
}

func TestGatewayAnthropicAPIOnlyOfficialClientCancellationContract(t *testing.T) {
	binary := os.Getenv("CLAUDE_GATEWAY_CONTRACT_BINARY")
	if binary == "" {
		t.Skip("set CLAUDE_GATEWAY_CONTRACT_BINARY to the qualified official Claude Code binary")
	}
	require.True(t, filepath.IsAbs(binary))
	root := t.TempDir()
	streaming := make(chan struct{})
	disconnected := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	f := newAnthropicRoutingContract(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_cancel\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\n")
		w.(http.Flusher).Flush()
		close(streaming)
		<-r.Context().Done()
		close(disconnected)
	})
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "CLAUDE_CONFIG_DIR=" + filepath.Join(root, "claude"), "ANTHROPIC_CONFIG_DIR=" + filepath.Join(root, "anthropic"), "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "ANTHROPIC_API_KEY=fixture-downstream-key", "ANTHROPIC_BASE_URL=" + f.server.URL + "/anthropic-api"}
	versionCtx, versionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer versionCancel()
	version := exec.CommandContext(versionCtx, binary, "--version")
	version.Env = env
	version.Dir = root
	output, err := version.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Equal(t, "2.1.300 (Claude Code)", strings.TrimSpace(string(output)))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "--bare", "--restricted", "-p", "Disposable cancellation fixture.", "--model", "claude-sonnet-4-6", "--tools", "", "--strict-mcp-config", "--setting-sources", "", "--no-session-persistence", "--max-turns", "1")
	command.Env = env
	command.Dir = root
	require.NoError(t, command.Start())
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	select {
	case <-streaming:
	case <-time.After(10 * time.Second):
		cancel()
		<-finished
		t.Fatal("official client did not open the real handler stream")
	}
	cancel()
	require.Error(t, <-finished)
	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("official client cancellation did not reach the loopback provider")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls, "client and gateway must not replay the canceled stream")
	t.Log("Actual Claude Code process cancellation propagated through the real Go API-only handler to the loopback provider without replay")
}
