// Package correlation carries transport-neutral request, trace, and event IDs.
package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	RequestHeader      = "X-Request-ID"
	TraceHeader        = "X-Trace-ID"
	RequestMetadataKey = "x-request-id"
	TraceMetadataKey   = "x-trace-id"
	RequestEventHeader = "request_id"
	TraceEventHeader   = "trace_id"
	EventHeader        = "event_id"
	maxIDLength        = 128
)

type IDs struct {
	RequestID string
	TraceID   string
	EventID   string
}

type contextKey struct{}

func Ensure(ctx context.Context, requestID, traceID string) (context.Context, IDs) {
	requestID = normalize(requestID)
	if requestID == "" {
		requestID = generateID()
	}
	traceID = normalize(traceID)
	if traceID == "" {
		traceID = requestID
	}
	ids := IDs{RequestID: requestID, TraceID: traceID}
	return WithContext(ctx, ids), ids
}

func WithContext(ctx context.Context, ids IDs) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ids.RequestID = normalize(ids.RequestID)
	ids.TraceID = normalize(ids.TraceID)
	ids.EventID = normalize(ids.EventID)
	return context.WithValue(ctx, contextKey{}, ids)
}

func WithEventID(ctx context.Context, eventID string) context.Context {
	ids := FromContext(ctx)
	ids.EventID = normalize(eventID)
	return WithContext(ctx, ids)
}

func FromContext(ctx context.Context) IDs {
	if ctx == nil {
		return IDs{}
	}
	ids, _ := ctx.Value(contextKey{}).(IDs)
	return ids
}

func normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxIDLength {
		return ""
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("-_.:", char) {
			continue
		}
		return ""
	}
	return value
}

func generateID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	// crypto/rand failures are exceptional; retain a valid non-empty boundary.
	return "correlation-unavailable"
}
