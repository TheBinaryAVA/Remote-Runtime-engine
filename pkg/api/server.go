package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/metrics"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
)

// ServerConfig holds API server configuration options.
type ServerConfig struct {
	Addr          string
	MaxQueueDepth int64
}

// Server provides the HTTP REST and WebSocket API gateway.
type Server struct {
	cfg        ServerConfig
	queue      queue.JobQueue
	eventBus   events.EventBus
	stateStore store.StateStore
	httpServer *http.Server
}

// NewServer creates a new API Server instance.
func NewServer(cfg ServerConfig, q queue.JobQueue, bus events.EventBus, st store.StateStore) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = 500
	}

	s := &Server{
		cfg:        cfg,
		queue:      q,
		eventBus:   bus,
		stateStore: st,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/submissions", s.handleSubmissions)
	mux.HandleFunc("/api/v1/submissions/", s.handleSubmissionDetailOrWS)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.Handle("/metrics", metrics.Handler())

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return s
}

// ServeHTTP delegates to the internal router handler for testing and middleware chaining.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer.Handler.ServeHTTP(w, r)
}

// Start launches the HTTP server.
func (s *Server) Start() error {
	log.Printf("[API Gateway] Server listening on %s (MaxQueueDepth=%d)", s.cfg.Addr, s.cfg.MaxQueueDepth)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// SubmitRequest defines the JSON payload for POST /api/v1/submissions.
type SubmitRequest struct {
	Language       string           `json:"language"`
	Code           string           `json:"code"`
	TimeLimitMs    int64            `json:"time_limit_ms,omitempty"`
	MemoryLimitMB  int64            `json:"memory_limit_mb,omitempty"`
	CpuQuota       float64          `json:"cpu_quota,omitempty"`
	PidsLimit      int64            `json:"pids_limit,omitempty"`
	MaxOutputBytes int64            `json:"max_output_bytes,omitempty"`
	SandboxType    string           `json:"sandbox_type,omitempty"`
	TestCases      []queue.TestCase `json:"test_cases"`
}

// SubmitResponse defines the 202 Accepted response payload.
type SubmitResponse struct {
	SubmissionID string             `json:"submission_id"`
	Status       events.EventStatus `json:"status"`
	WsURL        string             `json:"ws_url"`
	EnqueuedAt   time.Time          `json:"enqueued_at"`
}

func (s *Server) handleSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		s.writeJSONError(w, http.StatusBadRequest, "code cannot be empty")
		return
	}

	// Validate language
	if _, err := languages.Get(req.Language); err != nil {
		s.writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Backpressure check
	depth, err := s.queue.QueueDepth(r.Context())
	if err == nil && depth >= s.cfg.MaxQueueDepth {
		w.Header().Set("Retry-After", "5")
		s.writeJSONError(w, http.StatusTooManyRequests, fmt.Sprintf("Queue capacity exceeded (current depth: %d, max: %d); please retry shortly", depth, s.cfg.MaxQueueDepth))
		return
	}

	submissionID := "sub-" + uuid.New().String()[:12]
	now := time.Now()

	// Initial State
	state := &store.SubmissionState{
		SubmissionID:   submissionID,
		Status:         events.StatusQueued,
		Verdict:        models.VerdictAccepted,
		Language:       req.Language,
		TotalTestCases: len(req.TestCases),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.stateStore.SaveState(r.Context(), state); err != nil {
		s.writeJSONError(w, http.StatusInternalServerError, "Failed to initialize submission state: "+err.Error())
		return
	}

	// Enqueue Job
	job := &queue.SubmissionJob{
		SubmissionID:   submissionID,
		Language:       req.Language,
		Code:           req.Code,
		TimeLimitMs:    req.TimeLimitMs,
		MemoryLimitMB:  req.MemoryLimitMB,
		CpuQuota:       req.CpuQuota,
		PidsLimit:      req.PidsLimit,
		MaxOutputBytes: req.MaxOutputBytes,
		SandboxType:    req.SandboxType,
		TestCases:      req.TestCases,
		EnqueuedAt:     now,
	}
	if err := s.queue.Enqueue(r.Context(), job); err != nil {
		s.writeJSONError(w, http.StatusInternalServerError, "Failed to enqueue submission: "+err.Error())
		return
	}

	// Publish Initial QUEUED event
	_ = s.eventBus.Publish(r.Context(), submissionID, &events.ExecutionEvent{
		SubmissionID: submissionID,
		Status:       events.StatusQueued,
		Timestamp:    now,
	})

	resp := SubmitResponse{
		SubmissionID: submissionID,
		Status:       events.StatusQueued,
		WsURL:        fmt.Sprintf("/api/v1/submissions/%s/ws", submissionID),
		EnqueuedAt:   now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSubmissionDetailOrWS(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket subscription request
	if strings.HasSuffix(r.URL.Path, "/ws") {
		s.HandleWebSocket(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract submission ID
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/submissions/")
	id = strings.TrimSpace(id)
	if id == "" {
		s.writeJSONError(w, http.StatusBadRequest, "missing submission id")
		return
	}

	state, err := s.stateStore.GetState(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeJSONError(w, http.StatusNotFound, "submission not found")
			return
		}
		s.writeJSONError(w, http.StatusInternalServerError, "failed to get submission: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	depth, _ := s.queue.QueueDepth(r.Context())
	resp := map[string]interface{}{
		"status":      "healthy",
		"queue_depth": depth,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
	})
}
