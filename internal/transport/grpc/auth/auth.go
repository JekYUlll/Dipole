package grpcauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	serviceMetadataKey = "x-dipole-caller-service"
	secretMetadataKey  = "x-dipole-service-token"
)

type Credentials struct {
	Service string
	Secret  string
}

func NewUnaryClientInterceptor(credentials Credentials) (grpc.UnaryClientInterceptor, error) {
	credentials.Service = strings.TrimSpace(credentials.Service)
	credentials.Secret = strings.TrimSpace(credentials.Secret)
	if credentials.Service == "" || credentials.Secret == "" {
		return nil, errors.New("grpc service credentials are required")
	}

	return func(ctx context.Context, method string, request, reply any, connection *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(
			ctx,
			serviceMetadataKey, credentials.Service,
			secretMetadataKey, credentials.Secret,
		)
		return invoke(ctx, method, request, reply, connection, options...)
	}, nil
}

func NewUnaryServerInterceptor(secret string, allowedCallers ...string) (grpc.UnaryServerInterceptor, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("grpc service secret is required")
	}
	allowed := make(map[string]struct{}, len(allowedCallers))
	for _, caller := range allowedCallers {
		if caller = strings.TrimSpace(caller); caller != "" {
			allowed[caller] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil, errors.New("at least one grpc caller service is required")
	}

	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incoming, ok := metadata.FromIncomingContext(ctx)
		if !ok || !matchesSecret(incoming.Get(secretMetadataKey), secret) {
			return nil, status.Error(codes.Unauthenticated, "service authentication failed")
		}
		callers := incoming.Get(serviceMetadataKey)
		if len(callers) != 1 {
			return nil, status.Error(codes.Unauthenticated, "caller service identity is required")
		}
		if _, ok := allowed[strings.TrimSpace(callers[0])]; !ok {
			return nil, status.Error(codes.PermissionDenied, "caller service is not allowed")
		}
		return handler(ctx, request)
	}, nil
}

func matchesSecret(values []string, expected string) bool {
	if len(values) != 1 {
		return false
	}
	provided := strings.TrimSpace(values[0])
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
