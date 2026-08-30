package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
)

func TestMemoryEventBus_PubSub(t *testing.T) {
	bus := events.NewMemoryEventBus()
	ctx := context.Background()
	submissionID := "sub-test-123"

	ch, unsubscribe, err := bus.Subscribe(ctx, submissionID)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer unsubscribe()

	event := &events.ExecutionEvent{
		SubmissionID: submissionID,
		Status:       events.StatusQueued,
		Timestamp:    time.Now(),
	}

	if err := bus.Publish(ctx, submissionID, event); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	select {
	case received := <-ch:
		if received.Status != events.StatusQueued || received.SubmissionID != submissionID {
			t.Fatalf("unexpected event: %+v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestMemoryEventBus_MultipleSubscribers(t *testing.T) {
	bus := events.NewMemoryEventBus()
	ctx := context.Background()
	submissionID := "sub-multi-123"

	ch1, unsub1, _ := bus.Subscribe(ctx, submissionID)
	defer unsub1()
	ch2, unsub2, _ := bus.Subscribe(ctx, submissionID)
	defer unsub2()

	event := &events.ExecutionEvent{
		SubmissionID: submissionID,
		Status:       events.StatusRunning,
	}

	_ = bus.Publish(ctx, submissionID, event)

	select {
	case e1 := <-ch1:
		if e1.Status != events.StatusRunning {
			t.Fatalf("e1 status mismatch")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscriber 1 timed out")
	}

	select {
	case e2 := <-ch2:
		if e2.Status != events.StatusRunning {
			t.Fatalf("e2 status mismatch")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("subscriber 2 timed out")
	}
}
