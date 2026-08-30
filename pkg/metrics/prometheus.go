package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// SubmissionsTotal tracks total submission evaluations partitioned by language, verdict, and sandbox.
	SubmissionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "speedcode",
			Name:      "submissions_total",
			Help:      "Total number of code submissions processed by the execution engine.",
		},
		[]string{"language", "verdict", "sandbox_backend"},
	)

	// QueueDepth tracks the current number of pending jobs in the submission queue.
	QueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "speedcode",
			Name:      "queue_depth",
			Help:      "Current count of submissions waiting in the task queue.",
		},
	)

	// ExecutionDuration tracks the distribution of wall-clock execution time.
	ExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "speedcode",
			Name:      "execution_duration_seconds",
			Help:      "Wall-clock execution duration in seconds.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0},
		},
		[]string{"language", "verdict"},
	)

	// CPUTimeDuration tracks the distribution of CPU process execution time.
	CPUTimeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "speedcode",
			Name:      "cpu_time_seconds",
			Help:      "Total CPU user and kernel execution duration in seconds.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0},
		},
		[]string{"language", "verdict"},
	)

	// PeakMemoryBytes tracks the distribution of physical memory consumed by sandboxes.
	PeakMemoryBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "speedcode",
			Name:      "peak_memory_bytes",
			Help:      "Peak physical RAM memory consumed in bytes.",
			Buckets:   []float64{1048576, 4194304, 16777216, 67108864, 134217728, 268435456, 536870912},
		},
		[]string{"language"},
	)

	// SecurityViolationsTotal tracks security violations and anomalous process terminations.
	SecurityViolationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "speedcode",
			Name:      "security_violations_total",
			Help:      "Count of security boundary breaches, blocked syscalls, and resource terminations.",
		},
		[]string{"violation_type"},
	)

	// ActiveWorkers tracks the number of worker threads currently executing jobs.
	ActiveWorkers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "speedcode",
			Name:      "active_workers",
			Help:      "Number of worker threads currently processing code submissions.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		SubmissionsTotal,
		QueueDepth,
		ExecutionDuration,
		CPUTimeDuration,
		PeakMemoryBytes,
		SecurityViolationsTotal,
		ActiveWorkers,
	)
}

// RecordSubmissionMetrics updates Prometheus counters and histograms upon submission completion.
func RecordSubmissionMetrics(language, verdict, sandbox string, wallTimeMs, cpuTimeMs, peakMemoryMB float64) {
	SubmissionsTotal.WithLabelValues(language, verdict, sandbox).Inc()
	ExecutionDuration.WithLabelValues(language, verdict).Observe(wallTimeMs / 1000.0)
	CPUTimeDuration.WithLabelValues(language, verdict).Observe(cpuTimeMs / 1000.0)
	PeakMemoryBytes.WithLabelValues(language).Observe(peakMemoryMB * 1024 * 1024)
}

// RecordViolation updates the security violation counter metric.
func RecordViolation(violationType string) {
	SecurityViolationsTotal.WithLabelValues(violationType).Inc()
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
