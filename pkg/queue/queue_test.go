package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
)

func TestMemoryQueue_FIFO(t *testing.T) {
	q := queue.NewMemoryQueue(10)
	ctx := context.Background()

	job1 := &queue.SubmissionJob{SubmissionID: "sub-1", Language: "python3", Code: "print(1)"}
	job2 := &queue.SubmissionJob{SubmissionID: "sub-2", Language: "cpp", Code: "int main(){}"}

	if err := q.Enqueue(ctx, job1); err != nil {
		t.Fatalf("failed to enqueue job1: %v", err)
	}
	if err := q.Enqueue(ctx, job2); err != nil {
		t.Fatalf("failed to enqueue job2: %v", err)
	}

	depth, err := q.QueueDepth(ctx)
	if err != nil || depth != 2 {
		t.Fatalf("expected queue depth 2, got %d (err: %v)", depth, err)
	}

	dJob1, err := q.Dequeue(ctx, 1*time.Second)
	if err != nil || dJob1.SubmissionID != "sub-1" {
		t.Fatalf("expected job1, got %v (err: %v)", dJob1, err)
	}

	dJob2, err := q.Dequeue(ctx, 1*time.Second)
	if err != nil || dJob2.SubmissionID != "sub-2" {
		t.Fatalf("expected job2, got %v (err: %v)", dJob2, err)
	}

	// Queue should now be empty
	_, err = q.Dequeue(ctx, 50*time.Millisecond)
	if err != queue.ErrQueueEmpty {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestMemoryQueue_Backpressure(t *testing.T) {
	q := queue.NewMemoryQueue(1)
	ctx := context.Background()

	job1 := &queue.SubmissionJob{SubmissionID: "sub-1", Language: "python3", Code: "print(1)"}
	job2 := &queue.SubmissionJob{SubmissionID: "sub-2", Language: "python3", Code: "print(2)"}

	if err := q.Enqueue(ctx, job1); err != nil {
		t.Fatalf("failed to enqueue job1: %v", err)
	}

	err := q.Enqueue(ctx, job2)
	if err != queue.ErrBackpressureExceeded {
		t.Fatalf("expected ErrBackpressureExceeded, got %v", err)
	}
}
