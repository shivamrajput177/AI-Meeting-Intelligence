package tracing_test

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/tracing"
)

func TestInit_EmptyEndpointIsANoOp(t *testing.T) {
	shutdown, err := tracing.Init(context.Background(), "test-service", "")
	if err != nil {
		t.Fatalf("Init with empty endpoint: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInit_WithEndpointSucceedsWithoutDialing(t *testing.T) {
	// otlptracehttp.New doesn't dial the collector at construction time —
	// only on the first actual span export — so this succeeds even though
	// nothing is listening at this address in a unit test.
	shutdown, err := tracing.Init(context.Background(), "test-service", "localhost:4318")
	if err != nil {
		t.Fatalf("Init with endpoint: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected a non-nil shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
