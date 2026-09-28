//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2DateBinOriginIgnoresSessionTimezone(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SET LOCAL TIME ZONE 'Asia/Shanghai'`)
	require.NoError(t, err)
	var bucket time.Time
	err = tx.QueryRowContext(ctx,
		`SELECT date_bin(INTERVAL '1 day', TIMESTAMPTZ '2026-09-28 12:00:00+00', `+channelMonitorV2DateBinOrigin+`)`).Scan(&bucket)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), bucket.UTC())
}
