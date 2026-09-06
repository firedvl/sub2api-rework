package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

const (
	openAIAutoWarmupPrompt         = "Reply with OK only."
	openAIAutoWarmupInput          = "OK"
	openAIAutoWarmupRequestLimit   = 1 << 20
	openAIAutoWarmupRequestTimeout = 15 * time.Second
)

func (s *OpenAIGatewayService) SendOpenAIAutoWarmup(ctx context.Context, accountID int64) (*OpenAIAutoWarmupResult, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, infraerrors.InternalServer("OPENAI_AUTO_WARMUP_NOT_CONFIGURED", "OpenAI Auto Warm-up is not configured")
	}
	account, err := s.validateOpenAIAutoWarmupDispatch(ctx, accountID)
	if err != nil {
		return nil, err
	}

	accessToken, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "refresh OpenAI credentials: %v", err)
	}
	model, err := s.resolveOpenAIAutoWarmupModel(ctx, account, accessToken)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":        model,
		"instructions": openAIAutoWarmupPrompt,
		"input":        openAIAutoWarmupInput,
	}
	transform := applyCodexOAuthTransformWithOptions(payload, codexOAuthTransformOptions{SkipDefaultInstructions: true})
	if transform.Error != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_AUTO_WARMUP_REQUEST_FAILED", "normalize request: %v", transform.Error)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_AUTO_WARMUP_REQUEST_FAILED", "encode request: %v", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, openAIAutoWarmupRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, chatgptCodexURL, bytes.NewReader(body))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_AUTO_WARMUP_REQUEST_FAILED", "create request: %v", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	authHeaders, err := s.buildOpenAIAuthenticationHeaders(requestCtx, account, accessToken)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "build authentication: %v", err)
	}
	for key, values := range authHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if err := resolveAndSetOpenAIChatGPTAccountHeaders(requestCtx, s.accountRepo, req.Header, account); err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_AUTH_FAILED", "resolve ChatGPT account identity: %v", err)
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	ensureCodexIdentityHeaders(req.Header)
	enforceCodexIdentityHeadersWithUA(req.Header, s.codexIdentityOverrideUA(account))

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if _, err := s.validateOpenAIAutoWarmupDispatch(requestCtx, accountID); err != nil {
		return &OpenAIAutoWarmupResult{Model: model}, err
	}
	started := time.Now()
	resp, err := s.doOpenAIUpstream(req, proxyURL, account)
	latency := time.Since(started)
	if err != nil {
		return &OpenAIAutoWarmupResult{Model: model, Latency: latency}, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_UPSTREAM_FAILED", "send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, openAIAutoWarmupRequestLimit+1))
	if err != nil {
		return &OpenAIAutoWarmupResult{Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")), Latency: latency}, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_FAILED", "read response: %v", err)
	}
	if len(responseBody) > openAIAutoWarmupRequestLimit {
		return &OpenAIAutoWarmupResult{Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")), Latency: latency}, infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_TOO_LARGE", "warm-up response exceeded the read limit")
	}
	snapshot := ParseCodexRateLimitHeaders(resp.Header)
	if snapshot != nil {
		s.updateCodexUsageSnapshot(requestCtx, account.ID, snapshot)
	}
	result := &OpenAIAutoWarmupResult{
		Model: model, RequestID: strings.TrimSpace(resp.Header.Get("x-request-id")), Latency: latency,
		UpstreamStatus: resp.StatusCode,
	}
	result.WindowStarted, result.Observed5hResetAt, result.Observed5hUsedPercent = openAIAutoWarmupWindowEvidence(account, snapshot, time.Now())
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		result.UpstreamErrorCode = truncateString(strings.TrimSpace(extractUpstreamErrorCode(responseBody)), 120)
		result.UpstreamErrorMessage = truncateString(logredact.RedactText(sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))), 512)
		if result.UpstreamErrorMessage == "" {
			result.UpstreamErrorMessage = http.StatusText(resp.StatusCode)
		}
		s.handleOpenAIAccountUpstreamError(requestCtx, account, resp.StatusCode, resp.Header, responseBody, model)
		return result, infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_UPSTREAM_REJECTED", "OpenAI warm-up request returned status %d: %s", resp.StatusCode, result.UpstreamErrorMessage)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return result, infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_UNEXPECTED", "OpenAI warm-up returned a non-streaming response")
	}
	finalResponse, ok := extractCodexFinalResponse(string(responseBody))
	if !ok {
		return result, infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_RESPONSE_UNEXPECTED", "OpenAI warm-up stream did not contain a completed response")
	}
	result.ResponseID = extractOpenAIResponseIDFromJSONBytes(finalResponse)
	if usage, ok := extractOpenAIUsageFromJSONBytes(finalResponse); ok {
		result.Usage = usage
	}
	return result, nil
}

func (s *OpenAIGatewayService) validateOpenAIAutoWarmupDispatch(ctx context.Context, accountID int64) (*Account, error) {
	if s.settingService == nil {
		return nil, infraerrors.Conflict("OPENAI_AUTO_WARMUP_DISABLED", "OpenAI Auto Warm-up is disabled")
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil || settings == nil || !settings.OpenAIAutoWarmupEnabled {
		return nil, infraerrors.Conflict("OPENAI_AUTO_WARMUP_DISABLED", "OpenAI Auto Warm-up is disabled")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, firstError(err, ErrAccountNotFound)
	}
	if err := validateOpenAIAutoWarmupAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

func validateOpenAIAutoWarmupAccount(account *Account) error {
	used5h, hasUsed5h := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent")
	if !ResolveOpenAIAutoWarmupEnabled(account) || !account.IsSchedulable() || account.RateLimitedAt != nil || account.RateLimitResetAt != nil ||
		!hasUsed5h || math.IsNaN(used5h) || math.IsInf(used5h, 0) || used5h < 0 || used5h >= 100 {
		return infraerrors.Conflict("OPENAI_AUTO_WARMUP_ACCOUNT_UNSAFE", "account is not eligible for Auto Warm-up")
	}
	return nil
}

func (s *OpenAIGatewayService) resolveOpenAIAutoWarmupModel(ctx context.Context, account *Account, accessToken string) (string, error) {
	refreshed := *account
	refreshed.Credentials = cloneOpenAIAutoResetExtra(account.Credentials)
	if refreshed.Credentials == nil {
		refreshed.Credentials = make(map[string]any)
	}
	if strings.TrimSpace(accessToken) != "" {
		refreshed.Credentials["access_token"] = accessToken
	}
	manifest, err := s.fetchCodexModelsManifestWithLastKnownGood(ctx, &refreshed, "")
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "fetch Codex models: %v", err)
	}
	if manifest == nil {
		return "", infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "Codex models manifest is invalid")
	}
	model, err := selectOpenAIAutoWarmupModel(manifest.Body)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED", "parse Codex models: %v", err)
	}
	if model == "" {
		return "", infraerrors.New(http.StatusBadGateway, "OPENAI_AUTO_WARMUP_MODEL_UNAVAILABLE", "Codex models manifest has no usable text model")
	}
	return model, nil
}

type openAIAutoWarmupModelCandidate struct {
	slug            string
	contextWindow   int64
	maxOutputTokens int64
	priority        int
	manifestOrder   int
}

// selectOpenAIAutoWarmupModel uses capability size as a conservative quota
// proxy because ChatGPT/Codex OAuth does not publish per-model quota weights.
// Known smaller context/output limits win, then upstream priority/order, then
// slug for deterministic behavior. It never hardcodes a model entitlement.
func selectOpenAIAutoWarmupModel(body []byte) (string, error) {
	var envelope struct {
		Models []struct {
			Slug             string   `json:"slug"`
			SupportedInAPI   *bool    `json:"supported_in_api"`
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
			ContextWindow    int64    `json:"context_window"`
			MaxOutputTokens  int64    `json:"max_output_tokens"`
			Priority         int      `json:"priority"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	candidates := make([]openAIAutoWarmupModelCandidate, 0, len(envelope.Models))
	for index, model := range envelope.Models {
		slug := strings.TrimSpace(model.Slug)
		if slug == "" || model.SupportedInAPI != nil && !*model.SupportedInAPI || isCodexDedicatedMediaModel(slug) ||
			!openAIAutoWarmupSupportsText(model.InputModalities) || !openAIAutoWarmupSupportsText(model.OutputModalities) {
			continue
		}
		candidates = append(candidates, openAIAutoWarmupModelCandidate{
			slug: slug, contextWindow: model.ContextWindow, maxOutputTokens: model.MaxOutputTokens,
			priority: model.Priority, manifestOrder: index,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if cmp := compareKnownPositive(a.contextWindow, b.contextWindow); cmp != 0 {
			return cmp < 0
		}
		if cmp := compareKnownPositive(a.maxOutputTokens, b.maxOutputTokens); cmp != 0 {
			return cmp < 0
		}
		if cmp := compareKnownPositive(int64(a.priority), int64(b.priority)); cmp != 0 {
			return cmp < 0
		}
		if a.manifestOrder != b.manifestOrder {
			return a.manifestOrder < b.manifestOrder
		}
		return a.slug < b.slug
	})
	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0].slug, nil
}

func openAIAutoWarmupSupportsText(modalities []string) bool {
	if len(modalities) == 0 {
		return true
	}
	for _, modality := range modalities {
		if strings.EqualFold(strings.TrimSpace(modality), "text") {
			return true
		}
	}
	return false
}

func compareKnownPositive(a, b int64) int {
	switch {
	case a > 0 && b <= 0:
		return -1
	case a <= 0 && b > 0:
		return 1
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func openAIAutoWarmupWindowEvidence(account *Account, snapshot *OpenAICodexUsageSnapshot, observedAt time.Time) (bool, string, *float64) {
	normalized := snapshot.Normalize()
	if normalized == nil || normalized.Window5hMinutes == nil || *normalized.Window5hMinutes != int(openAIAutoWarmupWindowLength/time.Minute) ||
		normalized.Reset5hSeconds == nil || *normalized.Reset5hSeconds < 0 {
		return false, "", nil
	}
	resetAt := codexSnapshotBaseTime(snapshot, observedAt).Add(time.Duration(*normalized.Reset5hSeconds) * time.Second).UTC()
	previousReset, err := parseTime(strings.TrimSpace(fmt.Sprint(account.Extra["codex_5h_reset_at"])))
	if err != nil || previousReset.IsZero() {
		return false, resetAt.Format(time.RFC3339), normalized.Used5hPercent
	}
	previousUsed, hasPreviousUsed := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent")
	started := !observedAt.Before(previousReset) || resetAt.Sub(previousReset) >= openAIAutoWarmupResetAdvance ||
		normalized.Used5hPercent != nil && hasPreviousUsed && *normalized.Used5hPercent > previousUsed
	return started, resetAt.Format(time.RFC3339), normalized.Used5hPercent
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
