package grpccommon

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
)

func TestCorrelationPreservesAuthenticatedMetadataFallback(t *testing.T) {
	t.Parallel()
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "metadata-request", TraceID: "metadata-trace"})
	ctx = Correlation(ctx, &commonv1.RequestContext{})
	if got := correlation.FromContext(ctx); got.RequestID != "metadata-request" || got.TraceID != "metadata-trace" {
		t.Fatalf("unexpected correlation: %+v", got)
	}
}

func TestRequestContextFromCarriesCorrelation(t *testing.T) {
	t.Parallel()
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1"})
	request := RequestContextFrom(ctx, "U1", "gateway")
	if request.GetRequestId() != "R1" || request.GetTraceId() != "T1" {
		t.Fatalf("unexpected request context: %+v", request)
	}
}
