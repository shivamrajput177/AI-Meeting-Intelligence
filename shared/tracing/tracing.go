// Package tracing is every service's OpenTelemetry bootstrap, per
// docs/architecture/observability-security.md §1: "All services export
// via OTLP/HTTP ... to a cluster-local OTel Collector". Exports via
// OTLP/HTTP specifically (not OTLP/gRPC) to keep this codebase's existing
// no-gRPC-anywhere rule consistent even for telemetry — the same reason
// docs/architecture/microservices.md gives for REST/JSON over gRPC
// everywhere else.
//
// Scope, stated plainly: Init wires real spans for inbound HTTP (via
// shared/httpserver's otelhttp middleware) and outbound internal
// service-to-service calls (via shared/httpclient's otelhttp transport) —
// the two chokepoints every service already shares, so this one package
// change reaches all 11 of them. It does NOT instrument the Postgres
// driver (otelpgx) or add manual span propagation across Kafka producer/
// consumer headers, both named in observability-security.md §1 — those
// touch every repository/consumer file individually rather than a shared
// chokepoint, a larger and more invasive change this phase doesn't take
// on. A trace through this codebase today shows the HTTP legs of a
// request; it doesn't yet show the DB query or the Kafka hop that
// continues the same logical flow.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init sets the global TracerProvider and text-map propagator for the
// calling process, exporting spans via OTLP/HTTP to collectorEndpoint
// (e.g. "otel-collector.observability.svc.cluster.local:4318" — host:port,
// no scheme, matching otlptracehttp's own WithEndpoint contract). Head-based
// 100% sampling, per observability-security.md §1 ("head-based 100% in
// dev, tail-based ... documented for prod via the Collector's
// tailsampling processor" — sampling belongs at the Collector for a real
// deployment, not here).
//
// Call once at the top of main(), after loading config (collectorEndpoint
// comes from it) and before constructing httpserver.New — every service
// follows the same shape, see e.g. auth-service/main.go. The returned
// shutdown func flushes any buffered spans; defer it right after a
// successful Init.
func Init(ctx context.Context, serviceName, collectorEndpoint string) (shutdown func(context.Context) error, err error) {
	if collectorEndpoint == "" {
		// No collector configured (e.g. local dev without the Phase 6
		// observability stack running) — return a no-op shutdown rather
		// than failing startup. otel's own global TracerProvider default
		// is already a no-op, so simply not calling SetTracerProvider
		// here is enough; every otelhttp-wrapped call site still works,
		// it just never produces a real span.
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(collectorEndpoint),
		otlptracehttp.WithInsecure(), // plaintext OTLP/HTTP to a cluster-local Collector — see NetworkPolicy, not TLS, as this design's intra-cluster boundary
	)
	if err != nil {
		return nil, fmt.Errorf("create otlp http exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceNameKey.String(serviceName),
	))
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C traceparent — the same header format shared/kafkax already hand-rolls for its own Kafka propagation, so a trace_id lines up either way
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
