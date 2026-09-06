//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type visionQualificationRepo struct {
	AccountRepository
	account           *Account
	saved             *OpenAIVisionQualificationReport
	saveErr           error
	getAfterUpdateErr error
	saveCalls         int
	updateCalls       int
}

func (r *visionQualificationRepo) GetByID(context.Context, int64) (*Account, error) {
	if r.updateCalls > 0 && r.getAfterUpdateErr != nil {
		return nil, r.getAfterUpdateErr
	}
	return r.account, nil
}

func (r *visionQualificationRepo) SaveOpenAIVisionQualificationReport(_ context.Context, accountID int64, report *OpenAIVisionQualificationReport) error {
	r.saveCalls++
	if accountID != r.account.ID {
		return fmt.Errorf("wrong account: %d", accountID)
	}
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = report
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	r.account.Extra[OpenAIVisionQualificationExtraKey] = report
	return nil
}

func (r *visionQualificationRepo) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	r.account = account
	return nil
}

type visionQualificationUpstream struct {
	responses  []*http.Response
	bodies     [][]byte
	accountIDs []int64
}

func (u *visionQualificationUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return nil, errorsNewUnexpectedDo()
}

func errorsNewUnexpectedDo() error { return fmt.Errorf("unexpected Do call") }

func (u *visionQualificationUpstream) DoWithTLS(req *http.Request, _ string, accountID int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.bodies = append(u.bodies, body)
	u.accountIDs = append(u.accountIDs, accountID)
	if len(u.responses) == 0 {
		return nil, fmt.Errorf("no mocked response")
	}
	response := u.responses[0]
	u.responses = u.responses[1:]
	return response, nil
}

func visionQualificationAccount() *Account {
	return &Account{
		ID: 31, Name: "vision candidate", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-secret", "refresh_token": "refresh-secret",
			"chatgpt_account_id": "acct-stable", "chatgpt_user_id": "user-stable",
			openAIEndpointCapabilitiesCredentialKey: []any{"chat_completions", "alpha_search"},
		},
		Extra: map[string]any{},
	}
}

func visionAnswer(status int, answer string) *http.Response {
	body := fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\ndata: {\"type\":\"response.completed\"}\n\n", answer)
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func visionError(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func newVisionQualificationService(responses ...*http.Response) (*AccountTestService, *visionQualificationRepo, *visionQualificationUpstream) {
	repo := &visionQualificationRepo{account: visionQualificationAccount()}
	upstream := &visionQualificationUpstream{responses: responses}
	return &AccountTestService{accountRepo: repo, httpUpstream: upstream}, repo, upstream
}

func TestOpenAIVisionQualificationPreliminaryUsesExactAccountAndFixedCanary(t *testing.T) {
	svc, repo, upstream := newVisionQualificationService(visionAnswer(200, "1"), visionAnswer(200, "2"))
	report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
	require.NoError(t, err)
	require.Equal(t, VisionQualificationStatePreliminary, report.State)
	require.True(t, report.Preliminary.Passed)
	require.Equal(t, 2, report.Preliminary.Completed)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, []int64{31, 31}, upstream.accountIDs)
	require.Zero(t, repo.updateCalls, "qualification evidence must not use the scheduler-mutating account update path")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.bodies[0], &payload))
	require.Equal(t, OpenAIVisionQualificationModel, payload["model"])
	require.Equal(t, false, payload["store"])
	require.Equal(t, true, payload["stream"])
	require.NotContains(t, payload, "tools")
	input := payload["input"].([]any)
	require.Len(t, input, 1)
	content := input[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 2)
	imagePart := content[1].(map[string]any)
	require.Equal(t, "input_image", imagePart["type"])
	imageURL := imagePart["image_url"].(string)
	require.True(t, strings.HasPrefix(imageURL, "data:image/png;base64,"))
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(imageURL, "data:image/png;base64,"))
	require.NoError(t, err)
	require.True(t, isAllowlistedOpenAIVisionQualificationCanary(raw))
}

func TestOpenAIVisionQualificationGatesAndStopsOnFailure(t *testing.T) {
	t.Run("preliminary failure", func(t *testing.T) {
		svc, _, upstream := newVisionQualificationService(visionAnswer(200, "9"), visionAnswer(200, "2"))
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateIncorrectAnswer, report.State)
		require.Len(t, report.Preliminary.Attempts, 1)
		require.Len(t, upstream.bodies, 1)
	})

	t.Run("redacts non-numeric upstream answers", func(t *testing.T) {
		svc, _, _ := newVisionQualificationService(visionAnswer(200, "access-secret"))
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, "[redacted]", report.Preliminary.Attempts[0].VisualAnswer)
		require.NotContains(t, fmt.Sprintf("%+v", report), "access-secret")
	})

	t.Run("reliability ten of ten", func(t *testing.T) {
		responses := []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2")}
		for i := 0; i < 10; i++ {
			responses = append(responses, visionAnswer(200, fmt.Sprintf("%d", i%4+1)))
		}
		svc, _, upstream := newVisionQualificationService(responses...)
		preliminary, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.True(t, preliminary.Preliminary.Passed)
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStageReliability)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateQualified, report.State)
		require.True(t, report.Reliability.Passed)
		require.Equal(t, 10, report.Reliability.Completed)
		require.True(t, report.PromotionEligible)
		require.Len(t, upstream.bodies, 12)
	})

	t.Run("reliability failure", func(t *testing.T) {
		responses := []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2"), visionAnswer(200, "1"), visionAnswer(200, "9"), visionAnswer(200, "3")}
		svc, _, upstream := newVisionQualificationService(responses...)
		_, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStageReliability)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateIncorrectAnswer, report.State)
		require.Len(t, report.Reliability.Attempts, 2)
		require.Len(t, upstream.bodies, 4)
	})
}

func TestOpenAIVisionQualificationCooldownAnd429AreDeferred(t *testing.T) {
	t.Run("existing cooldown", func(t *testing.T) {
		svc, repo, upstream := newVisionQualificationService()
		until := time.Now().Add(time.Hour)
		repo.account.RateLimitResetAt = &until
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateCurrentlyUnavailable, report.State)
		require.Equal(t, visionFailureDeferred, report.Preliminary.Attempts[0].FailureClassification)
		require.Empty(t, upstream.bodies)
		require.Equal(t, &until, repo.account.RateLimitResetAt)
	})

	t.Run("upstream rate limit", func(t *testing.T) {
		svc, repo, _ := newVisionQualificationService(visionAnswer(http.StatusTooManyRequests, ""))
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateCurrentlyUnavailable, report.State)
		require.Equal(t, visionFailureDeferred, report.Preliminary.Attempts[0].FailureClassification)
		require.Nil(t, repo.account.RateLimitResetAt, "qualification must not write cooldown state")
	})
}

func TestOpenAIVisionQualificationRejectsIneligibleAccounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Account)
	}{
		{name: "non openai", mutate: func(a *Account) { a.Platform = PlatformAnthropic }},
		{name: "non oauth", mutate: func(a *Account) { a.Type = AccountTypeAPIKey }},
		{name: "disabled", mutate: func(a *Account) { a.Status = StatusDisabled }},
		{name: "unschedulable", mutate: func(a *Account) { a.Schedulable = false }},
		{name: "wrong mapping", mutate: func(a *Account) {
			a.Credentials["model_mapping"] = map[string]any{OpenAIVisionQualificationModel: "other-model"}
		}},
		{name: "missing stable identity", mutate: func(a *Account) {
			delete(a.Credentials, "chatgpt_account_id")
			delete(a.Credentials, "chatgpt_user_id")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, upstream := newVisionQualificationService()
			tc.mutate(repo.account)
			_, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
			require.Error(t, err)
			require.Empty(t, upstream.bodies)
		})
	}
}

func TestOpenAIVisionQualificationPromotionIsAdditiveAndKeepsSecretsAndCooldown(t *testing.T) {
	responses := []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2")}
	for i := 0; i < 10; i++ {
		responses = append(responses, visionAnswer(200, fmt.Sprintf("%d", i%4+1)))
	}
	svc, repo, _ := newVisionQualificationService(responses...)
	_, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
	require.NoError(t, err)
	_, err = svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStageReliability)
	require.NoError(t, err)
	cooldown := time.Now().Add(time.Hour)
	repo.account.RateLimitResetAt = &cooldown

	admin := &adminServiceImpl{accountRepo: repo}
	report, err := svc.PromoteOpenAIVisionQualification(context.Background(), 31, admin)
	require.NoError(t, err)
	require.NotNil(t, report.PromotedAt)
	require.False(t, report.PromotionEligible)
	require.Equal(t, 1, repo.updateCalls, "promotion must use the normal scheduler-refreshing account update path")
	require.Equal(t, "access-secret", repo.account.Credentials["access_token"])
	require.Equal(t, "refresh-secret", repo.account.Credentials["refresh_token"])
	require.Equal(t, []string{"alpha_search", "chat_completions", "vision_input"}, repo.account.Credentials[openAIEndpointCapabilitiesCredentialKey])
	require.Equal(t, &cooldown, repo.account.RateLimitResetAt)

	payload, err := json.Marshal(report)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "access-secret")
	require.NotContains(t, string(payload), "refresh-secret")
}

func TestOpenAIVisionQualificationPromotionRequiresRetainedGate(t *testing.T) {
	svc, repo, _ := newVisionQualificationService()
	_, err := svc.PromoteOpenAIVisionQualification(context.Background(), 31, &adminServiceImpl{accountRepo: repo})
	require.ErrorIs(t, err, ErrVisionQualificationPromotion)
	require.Zero(t, repo.updateCalls)
}

func TestAccountUpdateCannotAddVisionWithoutRetainedGatePromotion(t *testing.T) {
	repo := &visionQualificationRepo{account: visionQualificationAccount()}
	_, err := (&adminServiceImpl{accountRepo: repo}).UpdateAccount(context.Background(), repo.account.ID, &UpdateAccountInput{
		Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: []string{"chat_completions", "vision_input"}},
	})
	require.ErrorIs(t, err, ErrVisionQualificationPromotion)
	require.Zero(t, repo.updateCalls)
}

func TestAccountUpdateCannotForgeOrEraseVisionQualificationReport(t *testing.T) {
	repo := &visionQualificationRepo{account: visionQualificationAccount()}
	original := defaultOpenAIVisionQualificationReport(repo.account)
	original.State = VisionQualificationStatePreliminary
	repo.account.Extra[OpenAIVisionQualificationExtraKey] = original
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), repo.account.ID, &UpdateAccountInput{Extra: map[string]any{
		OpenAIVisionQualificationExtraKey: map[string]any{"state": VisionQualificationStateQualified, "promotion_eligible": true},
		"operator_note":                   "kept",
	}})

	require.NoError(t, err)
	require.Equal(t, original, updated.Extra[OpenAIVisionQualificationExtraKey])
	require.Equal(t, "kept", updated.Extra["operator_note"])
}

func TestOpenAIVisionQualificationBindsRetainedEvidenceToUpstreamIdentity(t *testing.T) {
	responses := []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2")}
	for i := 0; i < 10; i++ {
		responses = append(responses, visionAnswer(200, fmt.Sprintf("%d", i%4+1)))
	}
	svc, repo, upstream := newVisionQualificationService(responses...)
	delete(repo.account.Credentials, "chatgpt_user_id")
	_, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
	require.NoError(t, err)
	qualified, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStageReliability)
	require.NoError(t, err)
	fingerprint := qualified.UpstreamIdentityFingerprint
	require.NotEmpty(t, fingerprint)

	admin := &adminServiceImpl{accountRepo: repo}
	_, err = admin.UpdateAccount(context.Background(), 31, &UpdateAccountInput{Credentials: map[string]any{
		"access_token": "rotated-access", "refresh_token": "rotated-refresh",
		"chatgpt_account_id": "acct-stable", "chatgpt_user_id": "user-stable",
	}})
	require.NoError(t, err)
	retained, err := svc.GetOpenAIVisionQualification(context.Background(), 31)
	require.NoError(t, err)
	require.Equal(t, VisionQualificationStateQualified, retained.State)
	require.Equal(t, fingerprint, retained.UpstreamIdentityFingerprint)
	require.True(t, retained.PromotionEligible)
	require.False(t, retained.RequalificationRequired)

	_, err = admin.UpdateAccount(context.Background(), 31, &UpdateAccountInput{Credentials: map[string]any{
		"access_token": "other-access", "refresh_token": "other-refresh",
		"chatgpt_account_id": "acct-other", "chatgpt_user_id": "user-other",
	}})
	require.NoError(t, err)
	stale, err := svc.GetOpenAIVisionQualification(context.Background(), 31)
	require.NoError(t, err)
	require.Equal(t, VisionQualificationStateUnqualified, stale.State)
	require.False(t, stale.PromotionEligible)
	require.True(t, stale.RequalificationRequired)
	_, err = svc.PromoteOpenAIVisionQualification(context.Background(), 31, admin)
	require.ErrorIs(t, err, ErrVisionQualificationPromotion)

	upstream.responses = []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2")}
	replaced, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
	require.NoError(t, err)
	require.NotEqual(t, fingerprint, replaced.UpstreamIdentityFingerprint)
	require.False(t, replaced.RequalificationRequired)
	require.Nil(t, replaced.Reliability)

	payload, err := json.Marshal(replaced)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "acct-other")
	require.NotContains(t, string(payload), "user-other")
	require.NotContains(t, string(payload), "other-access")
	require.NotContains(t, string(payload), "other-refresh")
}

func TestOpenAIVisionQualificationClassifiesFailuresPrecisely(t *testing.T) {
	for _, tc := range []struct {
		name           string
		response       *http.Response
		classification string
		state          string
	}{
		{name: "unauthorized", response: visionError(401, `{"error":{"code":"invalid_token"}}`), classification: visionFailureAuthFailed, state: VisionQualificationStateAuthFailed},
		{name: "revoked token", response: visionError(401, `{"error":{"code":"token_revoked","message":"credential revoked"}}`), classification: visionFailureAuthFailed, state: VisionQualificationStateAuthFailed},
		{name: "forbidden", response: visionError(403, `{"error":{"code":"policy_denied"}}`), classification: visionFailureAuthOrPolicyDenied, state: VisionQualificationStateAuthOrPolicyDenied},
		{name: "rate limited", response: visionError(429, `{}`), classification: visionFailureDeferred, state: VisionQualificationStateCurrentlyUnavailable},
		{name: "service unavailable", response: visionError(503, `{}`), classification: visionFailureDeferred, state: VisionQualificationStateCurrentlyUnavailable},
		{name: "explicit image unsupported", response: visionError(400, `{"error":{"code":"image_input_unsupported","message":"image input is not supported"}}`), classification: visionFailureUnsupported, state: VisionQualificationStateUnsupported},
		{name: "other bad request", response: visionError(400, `{"error":{"code":"invalid_request"}}`), classification: visionFailureInvalidResponse, state: VisionQualificationStateInvalidResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newVisionQualificationService(tc.response)
			report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
			require.NoError(t, err)
			require.Equal(t, tc.state, report.State)
			require.Equal(t, tc.classification, report.Preliminary.Attempts[0].FailureClassification)
		})
	}

	t.Run("transport", func(t *testing.T) {
		svc, _, upstream := newVisionQualificationService()
		upstream.responses = nil
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateCurrentlyUnavailable, report.State)
		require.Equal(t, visionFailureTransportUnavailable, report.Preliminary.Attempts[0].FailureClassification)
	})

	t.Run("malformed success", func(t *testing.T) {
		svc, _, _ := newVisionQualificationService(visionError(200, `{"unexpected":true}`))
		report, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
		require.NoError(t, err)
		require.Equal(t, VisionQualificationStateInvalidResponse, report.State)
		require.Equal(t, visionFailureInvalidResponse, report.Preliminary.Attempts[0].FailureClassification)
	})
}

func TestAccountCreateAndBulkUpdateCannotAddVision(t *testing.T) {
	credentials := map[string]any{openAIEndpointCapabilitiesCredentialKey: []string{"chat_completions", "vision_input"}}
	_, err := (&adminServiceImpl{}).CreateAccount(context.Background(), &CreateAccountInput{
		Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: credentials, SkipDefaultGroupBind: true,
	})
	require.ErrorIs(t, err, ErrVisionQualificationPromotion)

	repo := &accountRepoStubForBulkUpdate{getByIDsAccounts: []*Account{{
		ID: 31, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: []string{"chat_completions"}},
	}, {
		ID: 32, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"chatgpt_account_id": "acct-32", openAIEndpointCapabilitiesCredentialKey: []string{"chat_completions"}},
	}}}
	_, err = (&adminServiceImpl{accountRepo: repo}).BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{31, 32}, Credentials: credentials,
	})
	require.ErrorIs(t, err, ErrVisionQualificationPromotion)
	require.Zero(t, repo.bulkUpdateCalls)
}

func TestAccountCredentialPersistenceCannotAddVision(t *testing.T) {
	repo := &visionQualificationRepo{account: visionQualificationAccount()}
	credentials := shallowCopyMap(repo.account.Credentials)
	credentials[openAIEndpointCapabilitiesCredentialKey] = []string{"chat_completions", "vision_input"}

	err := persistAccountCredentials(context.Background(), repo, repo.account, credentials)

	require.ErrorIs(t, err, ErrVisionQualificationPromotion)
	require.Zero(t, repo.updateCalls)
	require.False(t, hasConfiguredVisionCapability(repo.account.Credentials))
}

func TestDuplicateAccountCannotCopyVisionIntoNewAccount(t *testing.T) {
	repo := newDuplicateAccountRepoStub()
	source := visionQualificationAccount()
	source.Type = AccountTypeAPIKey
	source.Credentials = map[string]any{"api_key": "secret"}
	source.Credentials[openAIEndpointCapabilitiesCredentialKey] = []string{"chat_completions", "vision_input"}
	repo.accounts[source.ID] = source
	repo.mockAccountRepoForGemini.accountsByID[source.ID] = source

	_, err := (&adminServiceImpl{accountRepo: repo, accountDuplicateRepo: repo}).DuplicateAccount(
		context.Background(), source.ID, "admin:1", "",
	)

	require.ErrorIs(t, err, ErrVisionQualificationPromotion)
	require.Len(t, repo.accounts, 1)
}

func TestAccountUpdatePreservesOrRemovesExistingVision(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities []string
		hasVision    bool
	}{
		{name: "preserve", capabilities: []string{"chat_completions", "vision_input"}, hasVision: true},
		{name: "remove", capabilities: []string{"chat_completions"}, hasVision: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &visionQualificationRepo{account: visionQualificationAccount()}
			repo.account.Credentials[openAIEndpointCapabilitiesCredentialKey] = []string{"chat_completions", "vision_input"}
			updated, err := (&adminServiceImpl{accountRepo: repo}).UpdateAccount(context.Background(), 31, &UpdateAccountInput{
				Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: tc.capabilities},
			})
			require.NoError(t, err)
			require.Equal(t, tc.hasVision, hasConfiguredVisionCapability(updated.Credentials))
		})
	}
}

func TestOpenAIVisionQualificationPromotionPersistsCapabilityAndEvidenceTogether(t *testing.T) {
	responses := []*http.Response{visionAnswer(200, "1"), visionAnswer(200, "2")}
	for i := 0; i < 10; i++ {
		responses = append(responses, visionAnswer(200, fmt.Sprintf("%d", i%4+1)))
	}
	svc, repo, _ := newVisionQualificationService(responses...)
	_, err := svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStagePreliminary)
	require.NoError(t, err)
	_, err = svc.RunOpenAIVisionQualification(context.Background(), 31, VisionQualificationStageReliability)
	require.NoError(t, err)
	savesBeforePromotion := repo.saveCalls
	repo.saveErr = errorsNewUnexpectedDo()
	repo.getAfterUpdateErr = errorsNewUnexpectedDo()

	report, err := svc.PromoteOpenAIVisionQualification(context.Background(), 31, &adminServiceImpl{accountRepo: repo})
	require.NoError(t, err)
	require.True(t, hasConfiguredVisionCapability(repo.account.Credentials))
	require.NotNil(t, report.PromotedAt)
	require.Equal(t, report, repo.account.Extra[OpenAIVisionQualificationExtraKey])
	require.Equal(t, savesBeforePromotion, repo.saveCalls, "promotion must not persist evidence after the capability transaction")

	repo.getAfterUpdateErr = nil
	updateCalls := repo.updateCalls
	retried, err := svc.PromoteOpenAIVisionQualification(context.Background(), 31, &adminServiceImpl{accountRepo: repo})
	require.NoError(t, err)
	require.Equal(t, report.PromotedAt, retried.PromotedAt)
	require.Equal(t, updateCalls, repo.updateCalls, "completed promotion must be idempotent")
}
