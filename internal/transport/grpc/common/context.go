package grpccommon

import (
	"context"
	"strings"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RequestContext(principalUserID, callerService string) *commonv1.RequestContext {
	return RequestContextFrom(context.Background(), principalUserID, callerService)
}

func RequestContextFrom(ctx context.Context, principalUserID, callerService string) *commonv1.RequestContext {
	ids := correlation.FromContext(ctx)
	return &commonv1.RequestContext{
		PrincipalUserId: strings.TrimSpace(principalUserID),
		CallerService:   strings.TrimSpace(callerService),
		RequestId:       ids.RequestID,
		TraceId:         ids.TraceID,
	}
}

func Correlation(ctx context.Context, requestContext *commonv1.RequestContext) context.Context {
	if requestContext == nil {
		return ctx
	}
	existing := correlation.FromContext(ctx)
	requestID := strings.TrimSpace(requestContext.GetRequestId())
	if requestID == "" {
		requestID = existing.RequestID
	}
	traceID := strings.TrimSpace(requestContext.GetTraceId())
	if traceID == "" {
		traceID = existing.TraceID
	}
	ctx, _ = correlation.Ensure(ctx, requestID, traceID)
	return ctx
}

func Principal(requestContext *commonv1.RequestContext) (string, error) {
	if requestContext == nil || strings.TrimSpace(requestContext.GetPrincipalUserId()) == "" {
		return "", status.Error(codes.Unauthenticated, "principal_user_id is required")
	}
	return strings.TrimSpace(requestContext.GetPrincipalUserId()), nil
}

func Caller(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	if requestContext == nil || strings.TrimSpace(requestContext.GetCallerService()) == "" {
		return "", status.Error(codes.Unauthenticated, "caller_service is required")
	}
	claimed := strings.TrimSpace(requestContext.GetCallerService())
	if authenticated, ok := grpcauth.CallerService(ctx); ok && authenticated != claimed {
		return "", status.Error(codes.PermissionDenied, "caller_service does not match authenticated service")
	}
	return claimed, nil
}
