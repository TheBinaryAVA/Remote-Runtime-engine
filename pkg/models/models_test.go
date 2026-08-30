package models_test

import (
	"testing"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

func TestRequestNormalize(t *testing.T) {
	req := &models.ExecutionRequest{
		Language: "py",
	}
	req.Normalize()

	if req.ID == "" {
		t.Errorf("expected generated ID, got empty")
	}
	if req.Language != "python3" {
		t.Errorf("expected normalized language 'python3', got '%s'", req.Language)
	}
	if req.TimeLimitMs != models.DefaultTimeLimitMs {
		t.Errorf("expected default time limit %d, got %d", models.DefaultTimeLimitMs, req.TimeLimitMs)
	}
	if req.MemoryLimitMB != models.DefaultMemoryLimitMB {
		t.Errorf("expected default memory limit %d, got %d", models.DefaultMemoryLimitMB, req.MemoryLimitMB)
	}
	if req.PidsLimit != models.DefaultPidsLimit {
		t.Errorf("expected default pids limit %d, got %d", models.DefaultPidsLimit, req.PidsLimit)
	}
	if req.MaxOutputBytes != models.DefaultMaxOutputBytes {
		t.Errorf("expected default max output bytes %d, got %d", models.DefaultMaxOutputBytes, req.MaxOutputBytes)
	}
}
