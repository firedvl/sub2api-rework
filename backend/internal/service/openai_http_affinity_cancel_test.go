package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type responseBindContextProbeCache struct {
	stubGatewayCache
	contexts []context.Context
}

func (cache *responseBindContextProbeCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	cache.contexts = append(cache.contexts, ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	return cache.stubGatewayCache.SetSessionAccountID(ctx, groupID, sessionHash, accountID, ttl)
}

func TestBindHTTPResponseAccountSurvivesClientCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestContextKey := struct{}{}
	requestCtx, cancel := context.WithCancel(context.WithValue(context.Background(), requestContextKey, "caller"))
	cancel()
	for _, testCase := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "canceled", ctx: requestCtx},
		{name: "nil"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request, _ := gin.CreateTestContext(httptest.NewRecorder())
			request.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			groupID := int64(4201)
			request.Set("api_key", &APIKey{ID: 501, GroupID: &groupID})
			SetOpenAIHTTPResponseOwner(request, 601, 501)
			cache := &responseBindContextProbeCache{}
			svc := &OpenAIGatewayService{cache: cache}
			account := &Account{ID: 37001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
			startedAt := time.Now()
			svc.bindHTTPResponseAccount(testCase.ctx, request, account, "resp_canceled")
			require.Len(t, cache.contexts, 3)
			var sharedDeadline time.Time
			for _, bindingContext := range cache.contexts {
				deadline, ok := bindingContext.Deadline()
				require.True(t, ok)
				require.True(t, deadline.After(startedAt))
				require.LessOrEqual(t, deadline.Sub(startedAt), openAIWSStateStoreRedisTimeout+100*time.Millisecond)
				if sharedDeadline.IsZero() {
					sharedDeadline = deadline
				}
				require.Equal(t, sharedDeadline, deadline)
				if testCase.ctx != nil {
					require.Equal(t, "caller", bindingContext.Value(requestContextKey))
				}
			}
			require.Len(t, cache.sessionBindings, 3)
			accountID, err := svc.getOpenAIWSStateStore().GetResponseAccount(context.Background(), groupID, "resp_canceled")
			require.NoError(t, err)
			require.Equal(t, account.ID, accountID)
			owned, err := svc.ValidateOpenAIHTTPResponseOwner(context.Background(), groupID, "resp_canceled", 602, 501)
			require.NoError(t, err)
			require.False(t, owned)
		})
	}
}
