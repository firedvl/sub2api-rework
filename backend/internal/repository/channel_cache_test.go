package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestChannelCacheSubscriptionReceivesUpdateAndStops(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	bus := NewChannelCache(client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan struct{}, 10)
	finished := make(chan error, 1)
	go func() { finished <- bus.SubscribeUpdates(ctx, func() { updates <- struct{}{} }) }()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("subscription did not become ready")
	}
	require.NoError(t, bus.NotifyUpdate(context.Background()))
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("update was not delivered")
	}
	server.Close()
	require.NoError(t, server.Restart())
	select {
	case <-updates:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not invalidate potentially stale data")
	}
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("subscription did not stop")
	}
}
