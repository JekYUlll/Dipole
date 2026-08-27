package correlation

import (
	"context"
	"testing"
)

func TestEnsurePreservesValidIDsAndGeneratesMissingValues(t *testing.T) {
	t.Parallel()

	ctx, ids := Ensure(context.Background(), " request-1 ", "trace-1")
	if ids.RequestID != "request-1" || ids.TraceID != "trace-1" {
		t.Fatalf("unexpected preserved IDs: %+v", ids)
	}
	if got := FromContext(ctx); got != ids {
		t.Fatalf("context IDs = %+v, want %+v", got, ids)
	}

	_, generated := Ensure(context.Background(), "bad id with spaces", "")
	if generated.RequestID == "" || generated.RequestID == "bad id with spaces" {
		t.Fatalf("expected generated request ID, got %+v", generated)
	}
	if generated.TraceID != generated.RequestID {
		t.Fatalf("missing trace ID should inherit request ID: %+v", generated)
	}
}

func TestWithEventIDPreservesRequestAndTrace(t *testing.T) {
	t.Parallel()

	ctx := WithContext(context.Background(), IDs{RequestID: "R1", TraceID: "T1"})
	ctx = WithEventID(ctx, "E1")
	if got := FromContext(ctx); got != (IDs{RequestID: "R1", TraceID: "T1", EventID: "E1"}) {
		t.Fatalf("unexpected IDs: %+v", got)
	}
}
