package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/api"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/worker"
)

func TestAPISubmitAndGet(t *testing.T) {
	q := queue.NewMemoryQueue(100)
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()

	cfg := api.ServerConfig{MaxQueueDepth: 10}
	srv := api.NewServer(cfg, q, bus, st)

	// 1. Submit a job via POST /api/v1/submissions
	reqBody := api.SubmitRequest{
		Language:      "python3",
		Code:          "print('API Test')",
		TimeLimitMs:   2000,
		MemoryLimitMB: 128,
		SandboxType:   "dev_process",
	}
	bodyData, _ := json.Marshal(reqBody)

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", bytes.NewReader(bodyData))
	httpReq.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	var submitResp api.SubmitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &submitResp); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}

	if submitResp.SubmissionID == "" || submitResp.Status != events.StatusQueued {
		t.Fatalf("invalid submit response: %+v", submitResp)
	}

	// 2. Fetch state via GET /api/v1/submissions/{id}
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/submissions/"+submitResp.SubmissionID, nil)
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var state store.SubmissionState
	if err := json.Unmarshal(getRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to decode state: %v", err)
	}

	if state.SubmissionID != submitResp.SubmissionID || state.Status != events.StatusQueued {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestAPIHealth(t *testing.T) {
	q := queue.NewMemoryQueue(100)
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()

	srv := api.NewServer(api.ServerConfig{}, q, bus, st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", resp["status"])
	}
}

func TestAPIBackpressure(t *testing.T) {
	q := queue.NewMemoryQueue(2)
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()

	cfg := api.ServerConfig{MaxQueueDepth: 2}
	srv := api.NewServer(cfg, q, bus, st)

	// Fill queue to threshold (2 items)
	_ = q.Enqueue(context.Background(), &queue.SubmissionJob{SubmissionID: "sub-1", Language: "python3", Code: "print(1)"})
	_ = q.Enqueue(context.Background(), &queue.SubmissionJob{SubmissionID: "sub-2", Language: "python3", Code: "print(2)"})

	// Third request should be rejected with 429 Too Many Requests
	reqBody := api.SubmitRequest{
		Language:    "python3",
		Code:        "print('overflow')",
		SandboxType: "dev_process",
	}
	bodyData, _ := json.Marshal(reqBody)

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", bytes.NewReader(bodyData))
	srv.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 Too Many Requests, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Retry-After") != "5" {
		t.Fatalf("expected Retry-After: 5 header, got '%s'", rec.Header().Get("Retry-After"))
	}
}

func TestWebSocketStreaming(t *testing.T) {
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()
	q := queue.NewMemoryQueue(10)
	srv := api.NewServer(api.ServerConfig{MaxQueueDepth: 10}, q, bus, st)

	ts := httptest.NewServer(http.HandlerFunc(srv.HandleWebSocket))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/submissions/sub-stream-test/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Publish test event
	_ = bus.Publish(context.Background(), "sub-stream-test", &events.ExecutionEvent{
		SubmissionID: "sub-stream-test",
		Status:       events.StatusRunning,
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	var event events.ExecutionEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Status != events.StatusRunning || event.SubmissionID != "sub-stream-test" {
		t.Fatalf("unexpected streamed event: %+v", event)
	}
}

func TestConcurrentSubmissionProcessing(t *testing.T) {
	q := queue.NewMemoryQueue(100)
	bus := events.NewMemoryEventBus()
	st := store.NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker pool with 4 concurrent routines
	pool := worker.NewWorkerPool(worker.PoolConfig{Concurrency: 4, PollTimeout: 50 * time.Millisecond}, q, bus, st)
	pool.Start(ctx)
	defer pool.Stop()

	numSubmissions := 10
	var wg sync.WaitGroup
	errCh := make(chan error, numSubmissions)

	for i := 0; i < numSubmissions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			subID := fmt.Sprintf("sub-concurrent-%d", idx)

			job := &queue.SubmissionJob{
				SubmissionID:  subID,
				Language:      "python3",
				Code:          fmt.Sprintf("print(%d * 2)", idx),
				TimeLimitMs:   2000,
				MemoryLimitMB: 128,
				SandboxType:   "dev_process",
				TestCases: []queue.TestCase{
					{ID: "tc-1", Input: "", ExpectedOutput: fmt.Sprintf("%d\n", idx*2)},
				},
			}

			if err := q.Enqueue(ctx, job); err != nil {
				errCh <- fmt.Errorf("failed to enqueue %s: %w", subID, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrency error: %v", err)
	}

	// Wait for all submissions to complete
	deadline := time.Now().Add(5 * time.Second)
	completedCount := 0

	for time.Now().Before(deadline) {
		completedCount = 0
		for i := 0; i < numSubmissions; i++ {
			subID := fmt.Sprintf("sub-concurrent-%d", i)
			state, err := st.GetState(ctx, subID)
			if err == nil && state.Status == events.StatusCompleted && state.Verdict == models.VerdictAccepted {
				completedCount++
			}
		}
		if completedCount == numSubmissions {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if completedCount != numSubmissions {
		t.Fatalf("expected %d completed submissions, got %d", numSubmissions, completedCount)
	}
}
