package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const channelCachePubSubKey = "channel_cache_updated"

type channelCache struct{ rdb *redis.Client }

func NewChannelCache(rdb *redis.Client) service.ChannelCachePubSub {
	return &channelCache{rdb: rdb}
}

func (c *channelCache) NotifyUpdate(ctx context.Context) error {
	return c.rdb.Publish(ctx, channelCachePubSubKey, "refresh").Err()
}

func (c *channelCache) SubscribeUpdates(ctx context.Context, handler func()) error {
	pubsub := c.rdb.Subscribe(ctx, channelCachePubSubKey)
	defer func() { _ = pubsub.Close() }()
	if _, err := pubsub.ReceiveTimeout(ctx, 3*time.Second); err != nil {
		return fmt.Errorf("subscribe to channel cache invalidation: %w", err)
	}
	// Refresh after every subscription, including reconnects that may miss updates.
	handler()
	events := pubsub.ChannelWithSubscriptions()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return errors.New("channel cache subscription closed")
			}
			switch event.(type) {
			case *redis.Message, *redis.Subscription:
				handler()
			}
		}
	}
}
