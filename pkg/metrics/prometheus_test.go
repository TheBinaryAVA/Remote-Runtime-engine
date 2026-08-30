package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/metrics"
)

func TestPrometheusMetrics_Recording(t *testing.T) {
	metrics.RecordSubmissionMetrics("python3", "ACCEPTED", "native_cgroupv2", 150.0, 140.0, 4.5)
	metrics.RecordViolation("SECCOMP_SYSCALL_BLOCKED")

	handler := metrics.Handler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from /metrics, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "speedcode_submissions_total") {
		t.Fatalf("expected metric speedcode_submissions_total in output")
	}
	if !strings.Contains(body, "speedcode_execution_duration_seconds") {
		t.Fatalf("expected metric speedcode_execution_duration_seconds in output")
	}
	if !strings.Contains(body, "speedcode_security_violations_total") {
		t.Fatalf("expected metric speedcode_security_violations_total in output")
	}
}
