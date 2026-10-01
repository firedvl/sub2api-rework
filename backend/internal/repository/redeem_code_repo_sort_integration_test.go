//go:build integration

package repository

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *RedeemCodeRepoSuite) TestHistoryPaginationIsolationAndOrdering() {
	user := s.createUser("history@example.com")
	other := s.createUser("other-history@example.com")
	usedAt := time.Now().UTC().Truncate(time.Second)
	var ids []int64
	types := []string{service.RedeemTypeBalance, service.RedeemTypeConcurrency, service.RedeemTypeSubscription, "admin_balance", "admin_concurrency"}
	for index := 0; index < 105; index++ {
		itemTime := usedAt
		if index == 0 {
			itemTime = usedAt.Add(time.Hour)
		}
		code, err := s.client.RedeemCode.Create().SetCode(fmt.Sprintf("HISTORY-%d", index)).SetType(types[index%len(types)]).SetStatus(service.StatusUsed).SetUsedBy(user.ID).SetUsedAt(itemTime).Save(s.ctx)
		s.Require().NoError(err)
		ids = append(ids, code.ID)
	}
	_, err := s.client.RedeemCode.Create().SetCode("OTHER-HISTORY").SetType(service.RedeemTypeSubscription).SetStatus(service.StatusUsed).SetUsedBy(other.ID).SetUsedAt(usedAt.Add(2 * time.Hour)).Save(s.ctx)
	s.Require().NoError(err)
	expected := []int64{ids[0]}
	for index := len(ids) - 1; index > 0; index-- {
		expected = append(expected, ids[index])
	}
	for _, size := range []int{20, 50, 100} {
		var received []int64
		for page := 1; page <= (105+size-1)/size+1; page++ {
			codes, result, err := s.repo.ListByUserPaginated(s.ctx, user.ID, pagination.PaginationParams{Page: page, PageSize: size}, "")
			s.Require().NoError(err)
			s.Equal(int64(105), result.Total)
			s.LessOrEqual(len(codes), size)
			for _, code := range codes {
				s.Equal(user.ID, *code.UsedBy)
				received = append(received, code.ID)
			}
		}
		s.Equal(expected, received)
	}
	codes, result, err := s.repo.ListByUserPaginated(s.ctx, other.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, "")
	s.Require().NoError(err)
	s.Equal(int64(1), result.Total)
	s.Require().Len(codes, 1)
	s.Equal("OTHER-HISTORY", codes[0].Code)
	empty := s.createUser("empty-history@example.com")
	codes, result, err = s.repo.ListByUserPaginated(s.ctx, empty.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, "")
	s.Require().NoError(err)
	s.Empty(codes)
	s.Zero(result.Total)
}

func (s *RedeemCodeRepoSuite) TestListWithFilters_SortByValueAsc() {
	s.Require().NoError(s.repo.Create(s.ctx, &service.RedeemCode{Code: "VALUE-20", Type: service.RedeemTypeBalance, Value: 20, Status: service.StatusUnused}))
	s.Require().NoError(s.repo.Create(s.ctx, &service.RedeemCode{Code: "VALUE-10", Type: service.RedeemTypeBalance, Value: 10, Status: service.StatusUnused}))

	codes, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "value",
		SortOrder: "asc",
	}, "", "", "")
	s.Require().NoError(err)
	s.Require().Len(codes, 2)
	s.Require().Equal("VALUE-10", codes[0].Code)
	s.Require().Equal("VALUE-20", codes[1].Code)
}
