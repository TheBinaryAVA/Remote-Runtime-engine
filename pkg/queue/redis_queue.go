package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultRedisQueueKey = "speedcode:queue:submissions"
)

// RedisQueue implements the JobQueue interface backed by Redis lists with atomic operations.
type RedisQueue struct {
	client   *redis.Client
	queueKey string
}

// NewRedisQueue creates a new RedisQueue instance.
func NewRedisQueue(client *redis.Client, queueKey string) *RedisQueue {
	if queueKey == "" {
		queueKey = DefaultRedisQueueKey
	}
	return &RedisQueue{
		client:   client,
		queueKey: queueKey,
	}
}

// Enqueue serializes and pushes a job to the Redis queue.
func (r *RedisQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	if job == nil {
		return errors.New("cannot enqueue nil job")
	}
	job.Normalize()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	return r.client.LPush(ctx, r.queueKey, data).Err()
}

// Dequeue blocks until a job is available or until timeout occurs.
func (r *RedisQueue) Dequeue(ctx context.Context, timeout time.Duration) (*SubmissionJob, error) {
	res, err := r.client.BRPop(ctx, timeout, r.queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			return nil, ErrQueueEmpty
		}
		return nil, fmt.Errorf("redis dequeue failed: %w", err)
	}

	if len(res) < 2 {
		return nil, ErrQueueEmpty
	}

	var job SubmissionJob
	if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	return &job, nil
}

// QueueDepth returns the number of jobs waiting in the queue.
func (r *RedisQueue) QueueDepth(ctx context.Context) (int64, error) {
	return r.client.LLen(ctx, r.queueKey).Result()
}

// Close closes the Redis client connection.
func (r *RedisQueue) Close() error {
	return r.client.Close()
}

var _ JobQueue = (*RedisQueue)(nil)
