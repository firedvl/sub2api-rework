package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogObservationCompleteInsertAndScan(t *testing.T) {
	complete, incomplete := true, false
	for _, tt := range []struct {
		name  string
		value *bool
	}{{"unknown", nil}, {"incomplete", &incomplete}, {"complete", &complete}} {
		t.Run(tt.name, func(t *testing.T) {
			log := &service.UsageLog{
				UserID: 11, APIKeyID: 22, AccountID: 33,
				RequestID: "observation-" + tt.name, Model: "claude-sonnet-4",
				TotalCost: 0.4, ActualCost: 0.3, RateMultiplier: 1,
				CreatedAt: time.Now().UTC(), UsageObservationComplete: tt.value,
			}
			prepared := prepareUsageLogInsert(log)
			require.Len(t, prepared.args, len(usageLogInsertArgTypes))
			require.Equal(t, nullBool(tt.value), prepared.args[len(prepared.args)-2])
			require.Equal(t, "boolean", usageLogInsertArgTypes[len(prepared.args)-2])
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() {
				mock.ExpectClose()
				require.NoError(t, db.Close())
			})
			row := anySliceToDriverValues(append([]any{int64(44)}, prepared.args...))
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(strings.Split(usageLogSelectColumns, ", ")).AddRow(row...))
			rows, err := db.QueryContext(context.Background(), "SELECT "+usageLogSelectColumns)
			require.NoError(t, err)
			defer func() { require.NoError(t, rows.Close()) }()
			require.True(t, rows.Next())
			got, err := scanUsageLog(rows)
			require.NoError(t, err)
			require.Equal(t, tt.value, got.UsageObservationComplete)
			require.Equal(t, log.UserID, got.UserID)
			require.Equal(t, log.APIKeyID, got.APIKeyID)
			require.Equal(t, log.AccountID, got.AccountID)
			require.Equal(t, log.TotalCost, got.TotalCost)
			require.Equal(t, log.ActualCost, got.ActualCost)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
