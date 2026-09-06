package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	OpenAIVisionQualificationExtraKey = "openai_vision_qualification"
	OpenAIVisionQualificationModel    = "gpt-5.6-sol"
	OpenAIVisionQualificationEndpoint = "responses"

	VisionQualificationStagePreliminary = "preliminary"
	VisionQualificationStageReliability = "reliability"

	VisionQualificationStateUnqualified          = "UNQUALIFIED"
	VisionQualificationStatePreliminary          = "PRELIMINARY"
	VisionQualificationStateQualified            = "QUALIFIED"
	VisionQualificationStateCurrentlyUnavailable = "DEFERRED_CURRENTLY_UNAVAILABLE"
	VisionQualificationStateAuthFailed           = "AUTH_FAILED"
	VisionQualificationStateAuthOrPolicyDenied   = "AUTH_OR_POLICY_DENIED"
	VisionQualificationStateUnsupported          = "VISION_UNSUPPORTED"
	VisionQualificationStateIncorrectAnswer      = "INCORRECT_VISUAL_ANSWER"
	VisionQualificationStateInvalidResponse      = "INVALID_UPSTREAM_RESPONSE"
)

const (
	visionFailureNone                 = ""
	visionFailureDeferred             = "DEFERRED_CURRENTLY_UNAVAILABLE"
	visionFailureAuthFailed           = "AUTH_FAILED"
	visionFailureAuthOrPolicyDenied   = "AUTH_OR_POLICY_DENIED"
	visionFailureUnsupported          = "VISION_UNSUPPORTED"
	visionFailureIncorrectAnswer      = "INCORRECT_VISUAL_ANSWER"
	visionFailureInvalidResponse      = "INVALID_UPSTREAM_RESPONSE"
	visionFailureTransportUnavailable = "UPSTREAM_CURRENTLY_UNAVAILABLE"
)

var (
	ErrVisionQualificationInvalidAccount = infraerrors.BadRequest("VISION_QUALIFICATION_INVALID_ACCOUNT", "vision qualification requires an active, schedulable OpenAI OAuth account")
	ErrVisionQualificationModel          = infraerrors.BadRequest("VISION_QUALIFICATION_MODEL_UNAVAILABLE", "account does not support the fixed vision qualification model on Responses")
	ErrVisionQualificationIdentity       = infraerrors.BadRequest("VISION_QUALIFICATION_IDENTITY_UNAVAILABLE", "vision qualification requires a stable upstream ChatGPT account identity")
	ErrVisionQualificationPreliminary    = infraerrors.New(http.StatusConflict, "VISION_QUALIFICATION_PRELIMINARY_REQUIRED", "a retained 2/2 preliminary gate is required")
	ErrVisionQualificationPromotion      = infraerrors.New(http.StatusConflict, "VISION_QUALIFICATION_GATE_REQUIRED", "a retained 10/10 reliability gate is required before promotion")
	ErrVisionQualificationStorage        = errors.New("vision qualification storage is unavailable")
)

type OpenAIVisionQualificationAttempt struct {
	AccountID             int64     `json:"account_id"`
	AttemptID             string    `json:"attempt_id"`
	Timestamp             time.Time `json:"timestamp"`
	Model                 string    `json:"model"`
	Endpoint              string    `json:"endpoint"`
	UpstreamStatus        int       `json:"upstream_status"`
	VisualAnswer          string    `json:"visual_answer,omitempty"`
	ExpectedAnswer        string    `json:"expected_answer"`
	Correct               bool      `json:"correct"`
	LatencyMS             int64     `json:"latency_ms"`
	FailureClassification string    `json:"failure_classification,omitempty"`
}

type OpenAIVisionQualificationStageReport struct {
	Required  int                                `json:"required"`
	Completed int                                `json:"completed"`
	Passed    bool                               `json:"passed"`
	Attempts  []OpenAIVisionQualificationAttempt `json:"attempts"`
}

type OpenAIVisionQualificationReport struct {
	AccountID                   int64                                 `json:"account_id"`
	UpstreamIdentityFingerprint string                                `json:"upstream_identity_fingerprint,omitempty"`
	State                       string                                `json:"state"`
	Model                       string                                `json:"model"`
	Endpoint                    string                                `json:"endpoint"`
	Preliminary                 *OpenAIVisionQualificationStageReport `json:"preliminary,omitempty"`
	Reliability                 *OpenAIVisionQualificationStageReport `json:"reliability,omitempty"`
	QualifiedAt                 *time.Time                            `json:"qualification_timestamp,omitempty"`
	PromotionEligible           bool                                  `json:"promotion_eligible"`
	PromotedAt                  *time.Time                            `json:"promoted_at,omitempty"`
	RequalificationRequired     bool                                  `json:"requalification_required"`
	CurrentUnavailableUntil     *time.Time                            `json:"current_unavailable_until,omitempty"`
}

type openAIVisionQualificationReportRepository interface {
	SaveOpenAIVisionQualificationReport(context.Context, int64, *OpenAIVisionQualificationReport) error
}

type openAIVisionQualificationPromotionContextKey struct{}

func withOpenAIVisionQualificationPromotion(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIVisionQualificationPromotionContextKey{}, true)
}

// IsOpenAIVisionQualificationPromotion reports whether the service authorized
// this repository write as the retained-gate promotion operation.
func IsOpenAIVisionQualificationPromotion(ctx context.Context) bool {
	authorized, _ := ctx.Value(openAIVisionQualificationPromotionContextKey{}).(bool)
	return authorized
}

type openAIVisionCanary struct {
	png      []byte
	sha256   string
	expected string
}

var openAIVisionQualificationCanaries = buildOpenAIVisionQualificationCanaries()

func buildOpenAIVisionQualificationCanaries() []openAIVisionCanary {
	canaries := make([]openAIVisionCanary, 0, 4)
	for count := 1; count <= 4; count++ {
		img := image.NewRGBA(image.Rect(0, 0, 256, 128))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		const size, gap = 40, 12
		width := count*size + (count-1)*gap
		startX := (img.Bounds().Dx() - width) / 2
		for i := 0; i < count; i++ {
			x := startX + i*(size+gap)
			draw.Draw(img, image.Rect(x, 44, x+size, 84), image.NewUniform(color.RGBA{R: 220, G: 38, B: 38, A: 255}), image.Point{}, draw.Src)
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			panic(err)
		}
		raw := encoded.Bytes()
		hash := sha256.Sum256(raw)
		canaries = append(canaries, openAIVisionCanary{
			png: append([]byte(nil), raw...), sha256: hex.EncodeToString(hash[:]), expected: fmt.Sprintf("%d", count),
		})
	}
	return canaries
}

func isAllowlistedOpenAIVisionQualificationCanary(raw []byte) bool {
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	for _, canary := range openAIVisionQualificationCanaries {
		if len(raw) == len(canary.png) && fingerprint == canary.sha256 {
			return true
		}
	}
	return false
}

func openAIVisionQualificationIdentityFingerprint(account *Account) string {
	if account == nil || !account.IsOpenAIOAuthLike() {
		return ""
	}
	upstreamAccountID := strings.TrimSpace(account.GetChatGPTAccountID())
	if upstreamAccountID == "" {
		return ""
	}
	// chatgpt_user_id is optional metadata that token refresh may add later;
	// chatgpt_account_id is the stable upstream account identity.
	namespace := "chatgpt:" + upstreamAccountID
	digest := sha256.Sum256([]byte("sub2api:openai-vision-qualification-identity:v1:" + namespace))
	return hex.EncodeToString(digest[:])
}

func defaultOpenAIVisionQualificationReport(account *Account) *OpenAIVisionQualificationReport {
	return &OpenAIVisionQualificationReport{
		AccountID:                   account.ID,
		UpstreamIdentityFingerprint: openAIVisionQualificationIdentityFingerprint(account),
		State:                       VisionQualificationStateUnqualified,
		Model:                       OpenAIVisionQualificationModel,
		Endpoint:                    OpenAIVisionQualificationEndpoint,
	}
}

func openAIVisionQualificationReportFromAccount(account *Account) (*OpenAIVisionQualificationReport, bool) {
	if account == nil {
		return nil, false
	}
	report := defaultOpenAIVisionQualificationReport(account)
	raw, ok := account.Extra[OpenAIVisionQualificationExtraKey]
	if !ok || raw == nil {
		return report, true
	}
	payload, err := json.Marshal(raw)
	if err != nil || json.Unmarshal(payload, report) != nil || report.AccountID != account.ID ||
		report.Model != OpenAIVisionQualificationModel || report.Endpoint != OpenAIVisionQualificationEndpoint {
		return defaultOpenAIVisionQualificationReport(account), true
	}
	if current := openAIVisionQualificationIdentityFingerprint(account); current == "" || report.UpstreamIdentityFingerprint != current {
		report.State = VisionQualificationStateUnqualified
		report.PromotionEligible = false
		report.RequalificationRequired = true
		return report, false
	}
	report.RequalificationRequired = false
	return report, true
}

func (s *AccountTestService) GetOpenAIVisionQualification(ctx context.Context, accountID int64) (*OpenAIVisionQualificationReport, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := validateOpenAIVisionQualificationAccount(ctx, account); err != nil {
		return nil, err
	}
	report, _ := openAIVisionQualificationReportFromAccount(account)
	if unavailableUntil := openAIVisionQualificationUnavailableUntil(ctx, account, time.Now()); unavailableUntil != nil {
		report.CurrentUnavailableUntil = unavailableUntil
	}
	return report, nil
}

func (s *AccountTestService) RunOpenAIVisionQualification(ctx context.Context, accountID int64, stage string) (*OpenAIVisionQualificationReport, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := validateOpenAIVisionQualificationAccount(ctx, account); err != nil {
		return nil, err
	}

	report, identityMatches := openAIVisionQualificationReportFromAccount(account)
	required := 0
	switch stage {
	case VisionQualificationStagePreliminary:
		if !identityMatches {
			report = defaultOpenAIVisionQualificationReport(account)
		}
		required = 2
		report.Reliability = nil
		report.QualifiedAt = nil
		report.PromotedAt = nil
	case VisionQualificationStageReliability:
		if !identityMatches || report.Preliminary == nil || !report.Preliminary.Passed || report.Preliminary.Completed != 2 {
			return nil, ErrVisionQualificationPreliminary
		}
		required = 10
	default:
		return nil, infraerrors.BadRequest("VISION_QUALIFICATION_INVALID_STAGE", "stage must be preliminary or reliability")
	}

	stageReport := &OpenAIVisionQualificationStageReport{Required: required, Attempts: make([]OpenAIVisionQualificationAttempt, 0, required)}
	now := time.Now()
	if unavailableUntil := openAIVisionQualificationUnavailableUntil(ctx, account, now); unavailableUntil != nil {
		stageReport.Attempts = append(stageReport.Attempts, deferredOpenAIVisionQualificationAttempt(accountID, now, openAIVisionQualificationCanaries[0].expected))
		report.State = VisionQualificationStateCurrentlyUnavailable
		report.CurrentUnavailableUntil = unavailableUntil
	} else {
		report.CurrentUnavailableUntil = nil
		for i := 0; i < required; i++ {
			attempt := s.runOpenAIVisionQualificationAttempt(ctx, account, openAIVisionQualificationCanaries[i%len(openAIVisionQualificationCanaries)])
			stageReport.Attempts = append(stageReport.Attempts, attempt)
			if !attempt.Correct {
				break
			}
		}
		stageReport.Completed = len(stageReport.Attempts)
		stageReport.Passed = stageReport.Completed == required
		if stageReport.Passed {
			if stage == VisionQualificationStagePreliminary {
				report.State = VisionQualificationStatePreliminary
			} else {
				qualifiedAt := time.Now().UTC()
				report.State = VisionQualificationStateQualified
				report.QualifiedAt = &qualifiedAt
			}
		} else if len(stageReport.Attempts) > 0 && isDeferredVisionFailure(stageReport.Attempts[len(stageReport.Attempts)-1].FailureClassification) {
			report.State = VisionQualificationStateCurrentlyUnavailable
		} else {
			report.State = stageReport.Attempts[len(stageReport.Attempts)-1].FailureClassification
		}
	}
	stageReport.Completed = len(stageReport.Attempts)
	stageReport.Passed = stageReport.Completed == required

	if stage == VisionQualificationStagePreliminary {
		report.Preliminary = stageReport
	} else {
		report.Reliability = stageReport
	}
	report.PromotionEligible = report.State == VisionQualificationStateQualified && report.Reliability != nil && report.Reliability.Passed && report.Reliability.Completed == 10 && report.PromotedAt == nil
	if err := s.saveOpenAIVisionQualificationReport(ctx, accountID, report); err != nil {
		return nil, err
	}
	return report, nil
}

func validateOpenAIVisionQualificationAccount(ctx context.Context, account *Account) error {
	if account == nil || !account.IsOpenAIOAuth() || account.IsCredentialShadow() || !account.IsActive() || !account.Schedulable {
		return ErrVisionQualificationInvalidAccount
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !time.Now().Before(*account.ExpiresAt) {
		return ErrVisionQualificationInvalidAccount
	}
	if !account.IsModelSupported(OpenAIVisionQualificationModel) || account.GetMappedModel(OpenAIVisionQualificationModel) != OpenAIVisionQualificationModel ||
		!account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses) {
		return ErrVisionQualificationModel
	}
	if openAIVisionQualificationIdentityFingerprint(account) == "" {
		return ErrVisionQualificationIdentity
	}
	return nil
}

func openAIVisionQualificationUnavailableUntil(ctx context.Context, account *Account, now time.Time) *time.Time {
	var latest *time.Time
	for _, candidate := range []*time.Time{account.RateLimitResetAt, account.OverloadUntil, account.TempUnschedulableUntil} {
		if candidate != nil && candidate.After(now) && (latest == nil || candidate.After(*latest)) {
			value := candidate.UTC()
			latest = &value
		}
	}
	if remaining := account.GetModelRateLimitRemainingTimeWithContext(ctx, OpenAIVisionQualificationModel); remaining > 0 {
		value := now.Add(remaining).UTC()
		if latest == nil || value.After(*latest) {
			latest = &value
		}
	}
	return latest
}

func deferredOpenAIVisionQualificationAttempt(accountID int64, timestamp time.Time, expected string) OpenAIVisionQualificationAttempt {
	return OpenAIVisionQualificationAttempt{
		AccountID: accountID, AttemptID: uuid.NewString(), Timestamp: timestamp.UTC(),
		Model: OpenAIVisionQualificationModel, Endpoint: OpenAIVisionQualificationEndpoint,
		ExpectedAnswer: expected, FailureClassification: visionFailureDeferred,
	}
}

func (s *AccountTestService) runOpenAIVisionQualificationAttempt(ctx context.Context, account *Account, canary openAIVisionCanary) OpenAIVisionQualificationAttempt {
	started := time.Now()
	attempt := OpenAIVisionQualificationAttempt{
		AccountID: account.ID, AttemptID: uuid.NewString(), Timestamp: started.UTC(),
		Model: OpenAIVisionQualificationModel, Endpoint: OpenAIVisionQualificationEndpoint,
		ExpectedAnswer: canary.expected,
	}
	defer func() { attempt.LatencyMS = time.Since(started).Milliseconds() }()

	if !isAllowlistedOpenAIVisionQualificationCanary(canary.png) {
		attempt.FailureClassification = visionFailureUnsupported
		return attempt
	}
	if s.httpUpstream == nil {
		attempt.FailureClassification = visionFailureTransportUnavailable
		return attempt
	}

	payload := map[string]any{
		"model": OpenAIVisionQualificationModel,
		"input": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "Count the red squares in the image. Reply with exactly one digit and no other text."},
				{"type": "input_image", "image_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(canary.png), "detail": "low"},
			},
		}},
		"instructions":      "Return only the requested digit. Do not use tools.",
		"max_output_tokens": 16,
		"store":             false,
		"stream":            true,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		attempt.FailureClassification = visionFailureInvalidResponse
		return attempt
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(payloadBytes))
	if err != nil {
		attempt.FailureClassification = visionFailureTransportUnavailable
		return attempt
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if account.IsOpenAIAgentIdentity() {
		authHeaders, authErr := buildAgentIdentityAuthenticationHeaders(ctx, s.accountRepo, s.agentIdentityWS, &s.agentIdentityTaskMu, account)
		if authErr != nil {
			attempt.FailureClassification = visionFailureTransportUnavailable
			return attempt
		}
		for key, values := range authHeaders {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	} else {
		token := account.GetOpenAIAccessToken()
		if token == "" {
			attempt.FailureClassification = visionFailureAuthFailed
			return attempt
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	canonical := resolveCodexOutboundIdentity("")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Originator", canonical.originator)
	req.Header.Set("User-Agent", canonical.userAgent)
	setOpenAIChatGPTAccountHeaders(req.Header, account)
	enforceCodexIdentityHeadersWithUA(req.Header, account.GetOpenAIUserAgent())
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIAccountTestUpstream(req, proxyURL, account, true)
	if err != nil {
		attempt.FailureClassification = visionFailureTransportUnavailable
		return attempt
	}
	defer func() { _ = resp.Body.Close() }()
	attempt.UpstreamStatus = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if readErr != nil {
			attempt.FailureClassification = visionFailureInvalidResponse
			return attempt
		}
		attempt.FailureClassification = classifyOpenAIVisionUpstreamFailure(resp.StatusCode, body)
		return attempt
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		attempt.FailureClassification = visionFailureInvalidResponse
		return attempt
	}
	attempt.VisualAnswer, _ = parseOpenAIResponsesSSEForAlphaSearch(body)
	if strings.TrimSpace(attempt.VisualAnswer) == "" {
		attempt.VisualAnswer = extractOpenAIResponsesText(body)
	}
	rawAnswer := strings.TrimSpace(attempt.VisualAnswer)
	attempt.Correct = rawAnswer == attempt.ExpectedAnswer
	attempt.VisualAnswer = sanitizeOpenAIVisionAnswer(rawAnswer)
	if !attempt.Correct {
		if rawAnswer == "" {
			attempt.FailureClassification = visionFailureInvalidResponse
		} else {
			attempt.FailureClassification = visionFailureIncorrectAnswer
		}
	}
	return attempt
}

func classifyOpenAIVisionUpstreamFailure(status int, body []byte) string {
	if status == http.StatusUnauthorized {
		return visionFailureAuthFailed
	}
	if status == http.StatusTooManyRequests || status >= http.StatusInternalServerError {
		return visionFailureDeferred
	}
	if explicitlyRejectsOpenAIImageInput(body) {
		return visionFailureUnsupported
	}
	if status == http.StatusForbidden {
		return visionFailureAuthOrPolicyDenied
	}
	return visionFailureInvalidResponse
}

func explicitlyRejectsOpenAIImageInput(body []byte) bool {
	var payload struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(firstNonEmptyString(payload.Error.Code, payload.Code, payload.Error.Type, payload.Type)))
	switch code {
	case "image_input_unsupported", "unsupported_image_input", "vision_unsupported", "unsupported_vision", "multimodal_unsupported":
		return true
	}
	message := strings.ToLower(strings.TrimSpace(firstNonEmptyString(payload.Error.Message, payload.Message)))
	return strings.Contains(message, "does not support image input") ||
		strings.Contains(message, "image input is not supported") ||
		strings.Contains(message, "image inputs are not supported") ||
		strings.Contains(message, "unsupported image input")
}

func sanitizeOpenAIVisionAnswer(answer string) string {
	if answer == "" {
		return ""
	}
	if len(answer) > 8 {
		return "[redacted]"
	}
	for _, char := range answer {
		if char < '0' || char > '9' {
			return "[redacted]"
		}
	}
	return answer
}

func isDeferredVisionFailure(classification string) bool {
	return classification == visionFailureDeferred || classification == visionFailureTransportUnavailable
}

func (s *AccountTestService) saveOpenAIVisionQualificationReport(ctx context.Context, accountID int64, report *OpenAIVisionQualificationReport) error {
	repo, ok := s.accountRepo.(openAIVisionQualificationReportRepository)
	if !ok {
		return ErrVisionQualificationStorage
	}
	return repo.SaveOpenAIVisionQualificationReport(ctx, accountID, report)
}

func openAIVisionPromotionCapabilities(account *Account) (any, error) {
	configured, found := account.openAIEndpointCapabilitySet()
	if !found {
		configured = map[string]bool{
			string(OpenAIEndpointCapabilityChatCompletions): true,
			string(OpenAIEndpointCapabilityAlphaSearch):     account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAlphaSearch),
			string(OpenAIEndpointCapabilityLive):            account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityLive),
		}
	}
	if !configured[string(OpenAIEndpointCapabilityChatCompletions)] {
		return nil, ErrVisionQualificationModel
	}
	configured[string(OpenAIEndpointCapabilityVisionInput)] = true
	capabilities := make([]string, 0, len(configured))
	for capability, enabled := range configured {
		if enabled {
			capabilities = append(capabilities, capability)
		}
	}
	sort.Strings(capabilities)
	return capabilities, nil
}

func validateVisionCapabilityAddition(account *Account, platform, accountType string, credentials map[string]any, authorized bool) error {
	if credentials == nil || platform != PlatformOpenAI || account != nil && account.Platform == PlatformOpenAI && hasConfiguredVisionCapability(account.Credentials) {
		return nil
	}
	raw, provided := credentials[openAIEndpointCapabilitiesCredentialKey]
	if !provided {
		return nil
	}
	requested := &Account{Platform: platform, Type: accountType, Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: raw}}
	configured, found := requested.openAIEndpointCapabilitySet()
	if !found || !configured[string(OpenAIEndpointCapabilityVisionInput)] {
		return nil
	}
	if !authorized {
		return ErrVisionQualificationPromotion
	}
	return nil
}

func hasConfiguredVisionCapability(credentials map[string]any) bool {
	configured, found := (&Account{Credentials: credentials}).openAIEndpointCapabilitySet()
	return found && configured[string(OpenAIEndpointCapabilityVisionInput)]
}

func validateVisionCapabilityUpdate(account *Account, input *UpdateAccountInput) error {
	if account == nil || input == nil {
		return nil
	}
	if input.visionQualificationPromotion != nil && !validVisionQualificationPromotionReport(account, input.visionQualificationPromotion) {
		return ErrVisionQualificationPromotion
	}
	accountType := account.Type
	if input.Type != "" {
		accountType = input.Type
	}
	return validateVisionCapabilityAddition(account, account.Platform, accountType, input.Credentials, input.visionQualificationPromotion != nil)
}

func validVisionQualificationPromotionReport(account *Account, report *OpenAIVisionQualificationReport) bool {
	return account != nil && report != nil && report.AccountID == account.ID &&
		report.UpstreamIdentityFingerprint != "" && report.UpstreamIdentityFingerprint == openAIVisionQualificationIdentityFingerprint(account) &&
		report.Model == OpenAIVisionQualificationModel && report.Endpoint == OpenAIVisionQualificationEndpoint &&
		report.Reliability != nil && report.Reliability.Passed && report.Reliability.Completed == 10 &&
		report.QualifiedAt != nil && report.PromotedAt != nil
}

func (s *AccountTestService) PromoteOpenAIVisionQualification(ctx context.Context, accountID int64, admin AdminService) (*OpenAIVisionQualificationReport, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := validateOpenAIVisionQualificationAccount(ctx, account); err != nil {
		return nil, err
	}
	report, identityMatches := openAIVisionQualificationReportFromAccount(account)
	if !identityMatches || report.Reliability == nil || !report.Reliability.Passed || report.Reliability.Completed != 10 || report.QualifiedAt == nil {
		return nil, ErrVisionQualificationPromotion
	}
	if report.PromotedAt != nil {
		return report, nil
	}
	capabilities, err := openAIVisionPromotionCapabilities(account)
	if err != nil {
		return nil, err
	}
	promotedAt := time.Now().UTC()
	report.PromotedAt = &promotedAt
	report.PromotionEligible = false
	if _, err := admin.UpdateAccount(ctx, accountID, &UpdateAccountInput{
		Credentials:                  map[string]any{openAIEndpointCapabilitiesCredentialKey: capabilities},
		visionQualificationPromotion: report,
	}); err != nil {
		return nil, err
	}
	return report, nil
}
