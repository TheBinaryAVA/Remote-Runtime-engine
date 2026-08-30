package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
)

func TestMemoryStore_SaveAndGet(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	submissionID := "sub-store-1"

	state := &store.SubmissionState{
		SubmissionID:   submissionID,
		Status:         events.StatusRunning,
		Verdict:        models.VerdictAccepted,
		Language:       "python3",
		TotalTestCases: 3,
		CreatedAt:      time.Now(),
	}

	if err := st.SaveState(ctx, state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	retrieved, err := st.GetState(ctx, submissionID)
	if err != nil {
		t.Fatalf("failed to retrieve state: %v", err)
	}

	if retrieved.SubmissionID != submissionID || retrieved.Status != events.StatusRunning {
		t.Fatalf("retrieved state mismatch: %+v", retrieved)
	}

	// Test Not Found
	_, err = st.GetState(ctx, "non-existent")
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
