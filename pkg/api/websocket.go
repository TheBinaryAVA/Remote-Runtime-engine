package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow cross-origin WebSocket connections for competitive programming frontends
	},
}

// HandleWebSocket streams real-time execution events for a submission.
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract submission ID from path: /api/v1/submissions/:id/ws
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[len(pathParts)-1] != "ws" {
		http.Error(w, "invalid websocket route", http.StatusBadRequest)
		return
	}
	submissionID := pathParts[len(pathParts)-2]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS Upgrade Error] %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// If submission is already finished in store, send final state immediately
	if state, err := s.stateStore.GetState(ctx, submissionID); err == nil {
		if state.Status == events.StatusCompleted || state.Status == events.StatusFailed {
			finalEvent := &events.ExecutionEvent{
				SubmissionID:    state.SubmissionID,
				Status:          state.Status,
				Verdict:         state.Verdict,
				CurrentTestCase: state.TotalTestCases,
				TotalTestCases:  state.TotalTestCases,
				WallTimeMs:      state.TotalWallTimeMs,
				CpuTimeMs:       state.TotalCpuTimeMs,
				PeakMemoryMB:    state.PeakMemoryMB,
				Compilation:     state.Compilation,
				ErrorDetails:    state.ErrorDetails,
				Timestamp:       time.Now(),
			}
			_ = conn.WriteJSON(finalEvent)
			return
		}
	}

	eventCh, unsubscribe, err := s.eventBus.Subscribe(ctx, submissionID)
	if err != nil {
		log.Printf("[WS Subscribe Error] %v", err)
		_ = conn.WriteJSON(map[string]string{"error": "failed to subscribe to event bus"})
		return
	}
	defer unsubscribe()

	// Heartbeat ticker to maintain connection across NAT / reverse proxies
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	// Close listener goroutine
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case event, ok := <-eventCh:
			if !ok {
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

			// If final event, close connection cleanly
			if event.Status == events.StatusCompleted || event.Status == events.StatusFailed {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Execution Completed"))
				return
			}
		}
	}
}
