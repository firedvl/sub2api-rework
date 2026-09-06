package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type openAIVisionProbeRepoStub struct {
	AccountRepository
	account       *Account
	consumeCalled bool
	consumeID     int64
	consumeGroup  *int64
}

func (r *openAIVisionProbeRepoStub) ConsumeOpenAIVisionProbeCandidate(_ context.Context, accountID int64, groupID *int64) (bool, error) {
	r.consumeCalled = true
	r.consumeID = accountID
	r.consumeGroup = groupID
	return true, nil
}

func (r *openAIVisionProbeRepoStub) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

func TestParseOpenAIVisionProbeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set(OpenAIVisionProbeContractHeader, OpenAIVisionProbeContract)
	req.Header.Set(OpenAIVisionProbeAccountHeader, "31")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	accountID, requested, err := ParseOpenAIVisionProbeHeaders(c)
	if err != nil || !requested || accountID != 31 {
		t.Fatalf("parsed probe = id=%d requested=%v err=%v", accountID, requested, err)
	}
}

func TestClaimOpenAIVisionProbeSelectionConsumesOnlyExactCanary(t *testing.T) {
	raw := []byte("exact-test-canary")
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	oldHashes := openAIVisionProbeCanaryHashes
	openAIVisionProbeCanaryHashes = map[string]int64{hash: int64(len(raw))}
	defer func() { openAIVisionProbeCanaryHashes = oldHashes }()

	groupID := int64(7)
	repo := &openAIVisionProbeRepoStub{account: &Account{
		ID: 31, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}}
	service := &OpenAIGatewayService{accountRepo: repo}
	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","content":[{"type":"input_image","image_url":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(raw) + `"}]}]}`)

	selection, decision, err := service.ClaimOpenAIVisionProbeSelection(context.Background(), &groupID, 31, "gpt-5.6-sol", body)
	if err != nil {
		t.Fatal(err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 31 || !selection.Acquired {
		t.Fatalf("unexpected selection: %#v", selection)
	}
	if decision.Layer != "vision_probe" || decision.SelectedAccountID != 31 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if !repo.consumeCalled || repo.consumeID != 31 || repo.consumeGroup == nil || *repo.consumeGroup != groupID {
		t.Fatalf("unexpected consume call: id=%d group=%v", repo.consumeID, repo.consumeGroup)
	}
}

func TestClaimOpenAIVisionProbeSelectionRejectsArbitraryImageBeforeClaim(t *testing.T) {
	repo := &openAIVisionProbeRepoStub{}
	service := &OpenAIGatewayService{accountRepo: repo}
	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"input_image","image_url":"data:image/png;base64,cm9ibG94"}]}`)

	if _, _, err := service.ClaimOpenAIVisionProbeSelection(context.Background(), nil, 31, "gpt-5.6-sol", body); err == nil {
		t.Fatal("arbitrary image should be rejected")
	}
	if repo.consumeCalled {
		t.Fatal("candidate claim must not be consumed for an arbitrary image")
	}
}
