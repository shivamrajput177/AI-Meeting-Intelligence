// Package metrics is every service's Prometheus instrumentation, per
// docs/architecture/observability-security.md §1. It used to be a NoOp
// placeholder (Phase 1 through 5 had no Prometheus wiring at all); Phase
// 6 replaces it with the real thing rather than keeping an interface
// indirection nothing else in this codebase ever actually depended on —
// a grep across every service at the time of this change found zero call
// sites, so there was nothing to migrate.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler serves /metrics — shared/httpserver.New mounts this once per
// service, scraped by the ServiceMonitor CRD every service's own Helm
// chart already ships (deploy/helm/meeting-intel/charts/*/templates/servicemonitor.yaml,
// wired ahead of this in Phase 5 for exactly this moment).
func Handler() http.Handler {
	return promhttp.Handler()
}

// RED metrics — "Standard RED metrics per service" in
// observability-security.md §1. route is the net/http *pattern* a
// request matched (e.g. "GET /meetings/{id}"), never the raw URL path —
// shared/httpserver's own middleware resolves that via
// (*http.ServeMux).Handler before calling ObserveHTTPRequest, so a path
// parameter never explodes label cardinality the way raw paths would.
//
// The doc's own spec adds a `caller` label (gateway-forwarded vs. direct
// service-to-service) that isn't implemented here — distinguishing the
// two needs a signal shared/httpserver doesn't currently carry (which
// auth layer handled this particular request), and bolting one on just
// for this label would be a bigger, separate change. Named here as a
// real, scoped-out gap, not silently dropped.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests served, by method/route/status.",
	}, []string{"method", "route", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds, by method/route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)

// ObserveHTTPRequest records one request's RED metrics. Called from
// shared/httpserver's own middleware chain — the one place in every
// service that sees every request with both its matched route and final
// status code already resolved.
func ObserveHTTPRequest(method, route, status string, d time.Duration) {
	httpRequestsTotal.WithLabelValues(method, route, status).Inc()
	httpRequestDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

// Business metrics — the "Business metrics (custom)" row in
// observability-security.md §1. Each is incremented/observed from the
// one usecase that actually does the work it names (see
// meeting-service's UpdateStatusUseCase, transcription-service's
// ProcessUploadUseCase, ai-summary-service's ProcessTranscriptUseCase,
// action-item-service's ExtractActionItemsUseCase, and search-service's
// AskUseCase), not incremented generically from shared middleware the
// way the RED metrics above are — these are domain events, not HTTP
// events, and most of them (transcription, summarization, extraction)
// happen inside a Kafka consumer, never an HTTP handler at all.
var (
	MeetingsProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "meetings_processed_total",
		Help: "Meetings that reached the completed status.",
	})

	TranscriptionDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "transcription_duration_seconds",
		Help:    "Wall-clock time for one meeting's whisper.cpp transcription call.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s .. ~34min
	})

	LLMCallDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_call_duration_seconds",
		Help:    "Wall-clock time for one Ollama call, by model.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 12), // 0.5s .. ~17min
	}, []string{"model"})

	ActionItemsExtractedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "action_items_extracted_total",
		Help: "Action items persisted by extraction (including plain decision/risk/blocker rows).",
	})

	RAGQueryDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "rag_query_duration_seconds",
		Help:    "Wall-clock time for one POST /qa/ask round trip (retrieval + Ollama answer + persist).",
		Buckets: prometheus.DefBuckets,
	})

	// NotificationFailedTotal backs the "DLQ depth > 0" alert in
	// observability-security.md §1 — this codebase has no literal
	// dead-letter-queue topic (see notification-service's own
	// DispatchUseCase doc comments), so a permanently-failed row
	// (notification.failed.v1, published once MaxAttempts is exhausted)
	// is the real terminal-failure signal that alert is actually about.
	NotificationFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notification_failed_total",
		Help: "Outbox rows that permanently failed delivery (MaxAttempts exhausted), by channel.",
	}, []string{"channel"})
)
