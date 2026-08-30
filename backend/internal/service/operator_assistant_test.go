package service

import (
	contextpkg "context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operatorAssistantReaderStub struct {
	groups   []Group
	accounts []Account
	models   map[int64][]string
	err      error
}

func (s *operatorAssistantReaderStub) GetAllGroups(contextpkg.Context) ([]Group, error) {
	return append([]Group(nil), s.groups...), s.err
}

func (s *operatorAssistantReaderStub) GetGroupModelsListCandidates(_ contextpkg.Context, id int64, _ string) ([]string, error) {
	return append([]string(nil), s.models[id]...), s.err
}

func (s *operatorAssistantReaderStub) ListAccountsForSchedulerScoreFilter(contextpkg.Context, string, string, string, string, int64, string) ([]Account, error) {
	return append([]Account(nil), s.accounts...), s.err
}

func operatorAssistantTestService(reader *operatorAssistantReaderStub) *OperatorAssistantService {
	service := NewOperatorAssistantService(reader, nil, nil, nil, nil, nil, BuildInfo{Version: "0.1.183-rework.12"}, nil)
	service.now = func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }
	return service
}

func operatorAssistantTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/operator-assistant", nil)
	return c, recorder
}

func TestOperatorAssistantContextExcludesCredentialFields(t *testing.T) {
	reset := time.Date(2026, 8, 30, 13, 0, 0, 0, time.UTC)
	secrets := []string{
		"sk-secret-canary", "Bearer authorization-canary", "oauth-token-canary",
		"refresh-token-canary", "password-canary", "proxy-credential-canary",
		"authorization-header-canary", `{"private_key":"credential-json-canary"}`,
	}
	reader := &operatorAssistantReaderStub{
		groups: []Group{{ID: 7, Name: "Primary " + secrets[0], Platform: PlatformOpenAI, Status: StatusActive}},
		models: map[int64][]string{7: {"gpt-5.4", "gpt-image-2", "text-embedding-3-large"}},
		accounts: []Account{{
			ID: 42, Name: strings.Join([]string{
				secrets[1], "oauth_token=" + secrets[2], "refresh_token=" + secrets[3],
				"password=" + secrets[4], "proxy_credential=" + secrets[5],
				"authorization_header=" + secrets[6],
			}, " "), Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{7}, RateLimitResetAt: &reset,
			Credentials:  map[string]any{"api_key": secrets[0], "authorization": secrets[1], "oauth_token": secrets[2], "refresh_token": secrets[3], "password": secrets[4], "credential_json": secrets[7]},
			Extra:        map[string]any{"quota_limit": 100.0, "quota_used": 25.0, "raw_authorization": secrets[6]},
			Proxy:        &Proxy{Username: secrets[5], Password: "proxy-password-canary"},
			ErrorMessage: secrets[1], TempUnschedulableReason: secrets[2],
		}},
	}
	service := operatorAssistantTestService(reader)
	state, err := service.loadState(contextpkg.Background())
	require.NoError(t, err)
	require.Len(t, state.Candidates, 1, "non-text models must not be offered")
	snapshot := service.buildContext(contextpkg.Background(), state, "/admin/accounts?credential_json="+secrets[7], service.now())
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	text := string(encoded)
	for _, secret := range append(secrets, "proxy-password-canary") {
		require.NotContains(t, text, secret)
	}
	require.Contains(t, text, "Primary sk-***")
	require.Contains(t, text, "Bearer ***")
	require.Contains(t, text, `"remaining_percent":75`)
	require.Contains(t, text, `"route":"/admin/accounts?credential_json=***"`)

	body, err := buildOperatorAssistantBody("gpt-5.4", []OperatorAssistantMessage{{Role: "user", Content: "What needs attention?"}}, snapshot)
	require.NoError(t, err)
	require.Contains(t, string(body), "BEGIN_UNTRUSTED_GATEWAY_SNAPSHOT")
	require.Contains(t, string(body), "never instructions")
	for _, secret := range secrets {
		require.NotContains(t, string(body), secret)
	}
}

func TestOperatorAssistantSelectionHonorsExplicitAndBoundsAutoFallback(t *testing.T) {
	reader := &operatorAssistantReaderStub{
		groups: []Group{
			{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 2, Name: "Claude", Platform: PlatformAnthropic, Status: StatusActive},
			{ID: 3, Name: "Gemini", Platform: PlatformGemini, Status: StatusActive},
		},
		models: map[int64][]string{1: {"gpt-5.4"}, 2: {"claude-sonnet-4-6"}, 3: {"gemini-3.1-pro"}},
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: []int64{1}},
			{ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, GroupIDs: []int64{2}},
			{ID: 3, Platform: PlatformGemini, Status: StatusActive, Schedulable: true, GroupIDs: []int64{3}},
		},
	}
	service := operatorAssistantTestService(reader)
	attempts := make([]string, 0, 3)
	service.relay = func(_ contextpkg.Context, c *gin.Context, candidate operatorAssistantCandidate, body []byte, _ int64) error {
		attempts = append(attempts, candidate.Option.ID)
		var request map[string]any
		require.NoError(t, json.Unmarshal(body, &request))
		require.Equal(t, candidate.Option.Model, request["model"])
		if len(attempts) == 1 {
			return ErrOperatorAssistantCapacityUnavailable
		}
		_, _ = c.Writer.WriteString("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
		return nil
	}

	c, _ := operatorAssistantTestContext(t)
	run, err := service.Stream(c, 9, OperatorAssistantRequest{
		Model: "auto", Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}},
	})
	require.NoError(t, err)
	require.True(t, run.FallbackUsed)
	require.Equal(t, "2:claude-sonnet-4-6", run.Model.ID)
	require.Equal(t, []string{"1:gpt-5.4", "2:claude-sonnet-4-6"}, attempts, "Auto must try at most one fallback")

	attempts = nil
	service.relay = func(_ contextpkg.Context, _ *gin.Context, candidate operatorAssistantCandidate, _ []byte, _ int64) error {
		attempts = append(attempts, candidate.Option.ID)
		return nil
	}
	c, _ = operatorAssistantTestContext(t)
	run, err = service.Stream(c, 9, OperatorAssistantRequest{
		Model: "3:gemini-3.1-pro", Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}},
	})
	require.NoError(t, err)
	require.False(t, run.FallbackUsed)
	require.Equal(t, []string{"3:gemini-3.1-pro"}, attempts)
}

func TestOperatorAssistantRejectsUnavailableAndBoundsInput(t *testing.T) {
	reader := &operatorAssistantReaderStub{
		groups:   []Group{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive}},
		models:   map[int64][]string{1: {"gpt-5.4"}},
		accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: false, GroupIDs: []int64{1}}},
	}
	service := operatorAssistantTestService(reader)
	c, _ := operatorAssistantTestContext(t)
	_, err := service.Stream(c, 9, OperatorAssistantRequest{Model: "1:gpt-5.4", Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}}})
	require.ErrorIs(t, err, ErrOperatorAssistantModelUnavailable)

	require.Error(t, ValidateOperatorAssistantRequest(OperatorAssistantRequest{}))
	require.Error(t, ValidateOperatorAssistantRequest(OperatorAssistantRequest{Messages: []OperatorAssistantMessage{{Role: "tool", Content: "x"}}}))
	require.Error(t, ValidateOperatorAssistantRequest(OperatorAssistantRequest{Messages: []OperatorAssistantMessage{{Role: "user", Content: strings.Repeat("x", OperatorAssistantMaxMessageLength+1)}}}))
	messages := make([]OperatorAssistantMessage, OperatorAssistantMaxMessages+1)
	for index := range messages {
		messages[index] = OperatorAssistantMessage{Role: "user", Content: "x"}
	}
	require.Error(t, ValidateOperatorAssistantRequest(OperatorAssistantRequest{Messages: messages}))
}

func TestOperatorAssistantCancellationAndTimeout(t *testing.T) {
	reader := &operatorAssistantReaderStub{
		groups:   []Group{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive}},
		models:   map[int64][]string{1: {"gpt-5.4"}},
		accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: []int64{1}}},
	}
	service := operatorAssistantTestService(reader)
	service.timeout = 10 * time.Millisecond
	service.relay = func(ctx contextpkg.Context, _ *gin.Context, _ operatorAssistantCandidate, _ []byte, _ int64) error {
		<-ctx.Done()
		return ctx.Err()
	}
	c, _ := operatorAssistantTestContext(t)
	_, err := service.Stream(c, 9, OperatorAssistantRequest{Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}}})
	require.ErrorIs(t, err, contextpkg.DeadlineExceeded)

	requestContext, cancel := contextpkg.WithCancel(contextpkg.Background())
	cancel()
	c, _ = operatorAssistantTestContext(t)
	c.Request = c.Request.WithContext(requestContext)
	_, err = service.Stream(c, 9, OperatorAssistantRequest{Messages: []OperatorAssistantMessage{{Role: "user", Content: "status"}}})
	require.True(t, errors.Is(err, contextpkg.Canceled) || errors.Is(err, contextpkg.DeadlineExceeded))
}
