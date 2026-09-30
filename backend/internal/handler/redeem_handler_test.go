package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type redemptionHistoryRepo struct {
	service.RedeemCodeRepository
	userID      int64
	params      pagination.PaginationParams
	legacyLimit int
	codeType    string
}

func (repo *redemptionHistoryRepo) ListByUser(_ context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	repo.userID, repo.legacyLimit = userID, limit
	return []service.RedeemCode{}, nil
}

func (repo *redemptionHistoryRepo) ListByUserPaginated(_ context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	repo.userID, repo.params, repo.codeType = userID, params, codeType
	return []service.RedeemCode{}, &pagination.PaginationResult{Total: 101}, nil
}

func TestRedeemHistoryPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name, query        string
		userID             int64
		status, page, size int
	}{
		{"legacy", "", 7, 200, 0, 0},
		{"default", "?page=1", 7, 200, 1, 20},
		{"size only", "?page_size=50", 7, 200, 1, 50},
		{"caller isolation", "?page=2&page_size=100&user_id=7", 8, 200, 2, 100},
		{"cap", "?page_size=101", 7, 200, 1, 100},
		{"beyond last", "?page=100&page_size=20", 7, 200, 100, 20},
		{"zero", "?page=0", 7, 400, 0, 0},
		{"negative", "?page_size=-1", 7, 400, 0, 0},
		{"invalid", "?page=abc", 7, 400, 0, 0},
		{"empty", "?page=", 7, 400, 0, 0},
		{"offset overflow", "?page=" + strconv.FormatUint(uint64(^uint(0)>>1), 10), 7, 400, 0, 0},
		{"unauthenticated", "?page=1", 0, 401, 0, 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &redemptionHistoryRepo{}
			handler := NewRedeemHandler(service.NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil))
			recorder := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(recorder)
			requestContext.Request = httptest.NewRequest("GET", "/api/v1/redeem/history"+testCase.query, nil)
			if testCase.userID != 0 {
				requestContext.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: testCase.userID})
			}
			handler.GetHistory(requestContext)
			require.Equal(t, testCase.status, recorder.Code)
			if testCase.status != 200 {
				require.Zero(t, repo.userID)
				return
			}
			require.Equal(t, testCase.userID, repo.userID)
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			if testCase.query == "" {
				require.Equal(t, 25, repo.legacyLimit)
				require.JSONEq(t, "[]", string(body.Data))
				return
			}
			require.Equal(t, testCase.page, repo.params.Page)
			require.Equal(t, testCase.size, repo.params.PageSize)
			require.Empty(t, repo.codeType)
			var data struct {
				Items []service.RedeemCode `json:"items"`
				Total int                  `json:"total"`
				Page  int                  `json:"page"`
				Size  int                  `json:"page_size"`
			}
			require.NoError(t, json.Unmarshal(body.Data, &data))
			require.NotNil(t, data.Items)
			require.Equal(t, 101, data.Total)
			require.Equal(t, testCase.page, data.Page)
			require.Equal(t, testCase.size, data.Size)
		})
	}
}
