package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RedisEventChannelPrefix = "speedcode:events:"
)

// RedisEventBus implements EventBus backed by Redis Pub/Sub.
type RedisEventBus struct {
	client *redis.Client
}

// NewRedisEventBus creates a new RedisEventBus instance.
func NewRedisEventBus(client *redis.Client) *RedisEventBus {
	return &RedisEventBus{client: client}
}

// Publish broadcasts an event to the Redis channel for the given submission.
func (b *RedisEventBus) Publish(ctx context.Context, submissionID string, event *ExecutionEvent) error {
	if event == nil {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	event.SubmissionID = submissionID

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	channel := RedisEventChannelPrefix + submissionID
	return b.client.Publish(ctx, channel, data).Err()
}

// Subscribe listens on the Redis Pub/Sub channel for the submission.
func (b *RedisEventBus) Subscribe(ctx context.Context, submissionID string) (<-chan *ExecutionEvent, func(), error) {
	channel := RedisEventChannelPrefix + submissionID
	pubsub := b.client.Subscribe(ctx, channel)

	// Wait for subscription confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, fmt.Errorf("failed to subscribe to redis pubsub: %w", err)
	}

	eventCh := make(chan *ExecutionEvent, 64)
	var once sync.Once

	cleanup := func() {
		once.Do(func() {
			_ = pubsub.Close()
			close(eventCh)
		})
	}

	go func() {
		defer cleanup()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event ExecutionEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
					select {
					case eventCh <- &event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return eventCh, cleanup, nil
}

// Close closes the Redis client.
func (b *RedisEventBus) Close() error {
	return b.client.Close()
}

var _ EventBus = (*RedisEventBus)(nil)
