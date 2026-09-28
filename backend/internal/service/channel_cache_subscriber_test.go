//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type channelCacheBusStub struct {
	updates chan chan struct{}
	ready   chan struct{}
	notices atomic.Int32
}

func (b *channelCacheBusStub) NotifyUpdate(context.Context) error {
	b.notices.Add(1)
	return nil
}

func (b *channelCacheBusStub) SubscribeUpdates(ctx context.Context, handler func()) error {
	close(b.ready)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case done := <-b.updates:
			handler()
			close(done)
		}
	}
}

func TestChannelCacheSubscriberInvalidatesWithoutRepublishing(t *testing.T) {
	bus := &channelCacheBusStub{updates: make(chan chan struct{}, 1), ready: make(chan struct{})}
	first := NewChannelService(&mockChannelRepository{}, nil, nil, nil, bus)
	defer first.StopCacheSubscriber()
	<-bus.ready
	second := newTestChannelService(&mockChannelRepository{})
	_, err := first.buildCache(context.Background())
	require.NoError(t, err)
	_, err = second.buildCache(context.Background())
	require.NoError(t, err)
	second.cachePubSub = bus
	second.InvalidateCache()
	require.EqualValues(t, 1, bus.notices.Load())
	done := make(chan struct{})
	bus.updates <- done
	<-done
	require.Nil(t, first.cache.Load().(*channelCache))
	require.EqualValues(t, 1, bus.notices.Load())
	first.StopCacheSubscriber()
}

func TestChannelCacheInvalidationWaitsForOlderLoad(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		close(started)
		<-release
		return nil, nil
	}}
	svc := newTestChannelService(repo)
	loaded := make(chan error, 1)
	go func() { _, err := svc.buildCache(context.Background()); loaded <- err }()
	<-started
	cleared := make(chan struct{})
	go func() { svc.clearCache(); close(cleared) }()
	select {
	case <-cleared:
		t.Fatal("invalidation finished before the older load")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-loaded)
	<-cleared
	require.Nil(t, svc.cache.Load().(*channelCache))
}
