package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	OpenAIVisionProbeContractHeader = "X-Sub2API-Vision-Probe"
	OpenAIVisionProbeAccountHeader  = "X-Sub2API-Vision-Probe-Account"
	OpenAIVisionProbeContract       = "deterministic-pixel-canary-v1"
	openAIVisionProbeModel          = "gpt-5.6-sol"
)

var ErrOpenAIVisionProbeUnavailable = errors.New("HOSTED_VISION_UNAVAILABLE")

var openAIVisionProbeCanaryHashes = map[string]int64{
	"a39be4bdcccba5987ed3217ab9e699255dbc8f8fc3192038d0331a30da917ea0": 4127,
	"02aa724a44acfdf729889d55fe08de8157405406c3ff0ec4ffb1a2393c3a4430": 4037,
	"1ff4e7dace783fd8d90c2b86bc1472bddb918dfc846dc5fd87eb31165f9ae203": 4127,
	"f43f60b1e028434cb651f42c4e510fe7a420dfb6bce8bf0529dcd79d571fd64e": 4037,
}

func ParseOpenAIVisionProbeHeaders(c *gin.Context) (int64, bool, error) {
	if c == nil {
		return 0, false, nil
	}
	contract := strings.TrimSpace(c.GetHeader(OpenAIVisionProbeContractHeader))
	accountValue := strings.TrimSpace(c.GetHeader(OpenAIVisionProbeAccountHeader))
	if contract == "" && accountValue == "" {
		return 0, false, nil
	}
	if contract != OpenAIVisionProbeContract {
		return 0, true, ErrOpenAIVisionProbeUnavailable
	}
	accountID, err := strconv.ParseInt(accountValue, 10, 64)
	if err != nil || accountID <= 0 {
		return 0, true, ErrOpenAIVisionProbeUnavailable
	}
	return accountID, true, nil
}

func validateOpenAIVisionProbeCanary(requestedModel string, body []byte) error {
	if strings.TrimSpace(requestedModel) != openAIVisionProbeModel {
		return ErrOpenAIVisionProbeUnavailable
	}
	fingerprints := openAIVisionInputFingerprints(body)
	if len(fingerprints) != 1 {
		return ErrOpenAIVisionProbeUnavailable
	}
	fingerprint := fingerprints[0]
	expectedBytes, ok := openAIVisionProbeCanaryHashes[fingerprint.sha256]
	if !ok || fingerprint.representation != "data_url" || fingerprint.mimeType != "image/png" || fingerprint.byteLength != expectedBytes {
		return ErrOpenAIVisionProbeUnavailable
	}
	return nil
}

func (s *OpenAIGatewayService) ClaimOpenAIVisionProbeSelection(
	ctx context.Context,
	groupID *int64,
	accountID int64,
	requestedModel string,
	body []byte,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	decision := OpenAIAccountScheduleDecision{Layer: "vision_probe", CandidateCount: 1, TopK: 1}
	if s == nil || s.accountRepo == nil || validateOpenAIVisionProbeCanary(requestedModel, body) != nil {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	repo, ok := s.accountRepo.(OpenAIVisionProbeRepository)
	if !ok {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	claimed, err := repo.ConsumeOpenAIVisionProbeCandidate(ctx, accountID, groupID)
	if err != nil {
		return nil, decision, fmt.Errorf("consume vision probe candidate: %w", err)
	}
	if !claimed {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || !account.IsOpenAIOAuth() {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	if reason := openAICompatibleAccountEligibilityFailureReasonBeforeProfit(
		ctx, account, PlatformOpenAI, requestedModel, false, OpenAIEndpointCapabilityResponses,
	); reason != "" {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	acquired, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil {
		return nil, decision, fmt.Errorf("acquire vision probe account: %w", err)
	}
	decision.SelectedAccountID = account.ID
	decision.SelectedAccountType = account.Type
	if acquired != nil && acquired.Acquired {
		return &AccountSelectionResult{Account: account, Acquired: true, ReleaseFunc: acquired.ReleaseFunc}, decision, nil
	}
	if s.concurrencyService == nil {
		return nil, decision, ErrOpenAIVisionProbeUnavailable
	}
	cfg := s.schedulingConfig()
	return &AccountSelectionResult{
		Account: account,
		WaitPlan: &AccountWaitPlan{
			AccountID: account.ID, MaxConcurrency: account.Concurrency,
			Timeout: cfg.StickySessionWaitTimeout, MaxWaiting: cfg.StickySessionMaxWaiting,
		},
	}, decision, nil
}
