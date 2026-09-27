package kafkax

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

func TestNewTraceparent_WellFormed(t *testing.T) {
	tp := NewTraceparent()
	parts := strings.Split(tp, "-")
	if len(parts) != 4 {
		t.Fatalf("NewTraceparent() = %q, want 4 dash-separated segments", tp)
	}
	if parts[0] != "00" || parts[3] != "01" {
		t.Fatalf("NewTraceparent() = %q, want version=00 and flags=01", tp)
	}
	if len(parts[1]) != 32 {
		t.Fatalf("trace-id segment length = %d, want 32", len(parts[1]))
	}
	if len(parts[2]) != 16 {
		t.Fatalf("span-id segment length = %d, want 16", len(parts[2]))
	}
}

func TestNewTraceparent_UniquePerCall(t *testing.T) {
	first := NewTraceparent()
	second := NewTraceparent()
	if first == second {
		t.Fatalf("expected two calls to NewTraceparent to produce different values, both were %q", first)
	}
}

func TestChildTraceparent_PreservesTraceID(t *testing.T) {
	parent := NewTraceparent()
	child := ChildTraceparent(parent)

	if TraceIDOf(child) != TraceIDOf(parent) {
		t.Fatalf("ChildTraceparent changed the trace-id: parent=%q child=%q", parent, child)
	}
	if child == parent {
		t.Fatal("expected ChildTraceparent to mint a new span-id, not return the same value")
	}
}

func TestChildTraceparent_FallsBackToNewTraceOnGarbage(t *testing.T) {
	for _, bad := range []string{"", "not-a-traceparent", "00-tooshort-01"} {
		child := ChildTraceparent(bad)
		if TraceIDOf(child) == "" {
			t.Fatalf("ChildTraceparent(%q) = %q, want a well-formed fallback traceparent", bad, child)
		}
	}
}

func TestTraceIDOf_Invalid(t *testing.T) {
	for _, bad := range []string{"", "garbage", "00-shorttraceid-shortspanid-01"} {
		if got := TraceIDOf(bad); got != "" {
			t.Errorf("TraceIDOf(%q) = %q, want empty string", bad, got)
		}
	}
}

func TestTraceparentFromHeaders(t *testing.T) {
	tp := NewTraceparent()
	headers := []kafka.Header{{Key: "some-other-header", Value: []byte("x")}, {Key: HeaderTraceparent, Value: []byte(tp)}}
	if got := TraceparentFromHeaders(headers); got != tp {
		t.Errorf("TraceparentFromHeaders = %q, want %q", got, tp)
	}
	if got := TraceparentFromHeaders(nil); got != "" {
		t.Errorf("TraceparentFromHeaders(nil) = %q, want empty string", got)
	}
}

func TestTraceHeaders(t *testing.T) {
	tp := NewTraceparent()
	ctx := reqctx.WithTraceparent(context.Background(), tp)

	headers := traceHeaders(ctx)
	if len(headers) != 1 || headers[0].Key != HeaderTraceparent || string(headers[0].Value) != tp {
		t.Fatalf("traceHeaders(ctx with traceparent) = %+v, want one traceparent header with value %q", headers, tp)
	}

	if got := traceHeaders(context.Background()); got != nil {
		t.Errorf("traceHeaders(ctx without traceparent) = %+v, want nil", got)
	}
}
