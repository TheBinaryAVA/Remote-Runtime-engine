package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RedisStatePrefix = "speedcode:state:"
	DefaultStateTTL  = 24 * time.Hour
)

// RedisStore implements StateStore backed by Redis with automatic TTL expiration.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a new RedisStore instance.
func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = DefaultStateTTL
	}
	return &RedisStore{
		client: client,
		ttl:    ttl,
	}
}

// SaveState serializes and saves the submission state into Redis.
func (r *RedisStore) SaveState(ctx context.Context, state *SubmissionState) error {
	if state == nil {
		return errors.New("cannot save nil state")
	}
	state.UpdatedAt = time.Now()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal submission state: %w", err)
	}

	key := RedisStatePrefix + state.SubmissionID
	return r.client.Set(ctx, key, data, r.ttl).Err()
}

// GetState retrieves and deserializes the submission state from Redis.
func (r *RedisStore) GetState(ctx context.Context, submissionID string) (*SubmissionState, error) {
	key := RedisStatePrefix + submissionID
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get submission state: %w", err)
	}

	var state SubmissionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal submission state: %w", err)
	}

	return &state, nil
}

// Close closes the Redis client.
func (r *RedisStore) Close() error {
	return r.client.Close()
}

var _ StateStore = (*RedisStore)(nil)
