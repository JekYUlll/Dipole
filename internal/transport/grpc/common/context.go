package grpccommon

import (
	"strings"

	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RequestContext(principalUserID, callerService string) *commonv1.RequestContext {
	return &commonv1.RequestContext{
		PrincipalUserId: strings.TrimSpace(principalUserID),
		CallerService:   strings.TrimSpace(callerService),
	}
}

func Principal(requestContext *commonv1.RequestContext) (string, error) {
	if requestContext == nil || strings.TrimSpace(requestContext.GetPrincipalUserId()) == "" {
		return "", status.Error(codes.Unauthenticated, "principal_user_id is required")
	}
	return strings.TrimSpace(requestContext.GetPrincipalUserId()), nil
}

func Caller(requestContext *commonv1.RequestContext) (string, error) {
	if requestContext == nil || strings.TrimSpace(requestContext.GetCallerService()) == "" {
		return "", status.Error(codes.Unauthenticated, "caller_service is required")
	}
	return strings.TrimSpace(requestContext.GetCallerService()), nil
}
