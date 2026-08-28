package logger

import (
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

var queryKeyNormalizer = strings.NewReplacer("-", "_", ".", "_")

func GinLogger() gin.HandlerFunc {
	return ginLogger(L())
}

func ginLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactQuery(c.Request.URL.RawQuery)

		c.Next()

		if query != "" {
			path = path + "?" + query
		}

		fields := []zap.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", time.Since(start)),
			zap.Int("body_size", c.Writer.Size()),
		}
		ids := correlation.FromContext(c.Request.Context())
		fields = append(fields,
			zap.String("request_id", ids.RequestID),
			zap.String("trace_id", ids.TraceID),
		)
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		httpLogger := log.Named("http")
		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			httpLogger.Error("request completed", fields...)
		case c.Writer.Status() >= http.StatusBadRequest:
			httpLogger.Warn("request completed", fields...)
		default:
			httpLogger.Info("request completed", fields...)
		}
	}
}

func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}

	query, err := url.ParseQuery(raw)
	if err != nil {
		return "REDACTED"
	}
	for key, values := range query {
		if !sensitiveQueryKey(key) {
			continue
		}
		for index := range values {
			values[index] = "REDACTED"
		}
	}
	return query.Encode()
}

func sensitiveQueryKey(key string) bool {
	normalized := queryKeyNormalizer.Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "token", "access_token", "refresh_token", "id_token", "authorization",
		"api_key", "apikey", "x_api_key", "awsaccesskeyid", "googleaccessid",
		"client_secret", "password", "passwd", "secret", "credential", "signature", "sig":
		return true
	default:
		return strings.HasSuffix(normalized, "_token") ||
			strings.HasSuffix(normalized, "_secret") ||
			strings.HasSuffix(normalized, "_password") ||
			strings.HasSuffix(normalized, "_signature") ||
			strings.HasSuffix(normalized, "_credential")
	}
}

func GinRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		ids := correlation.FromContext(c.Request.Context())
		L().Named("http").Error("panic recovered",
			zap.Any("panic", recovered),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client_ip", c.ClientIP()),
			zap.String("request_id", ids.RequestID),
			zap.String("trace_id", ids.TraceID),
			zap.ByteString("stack", debug.Stack()),
		)

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "internal server error",
		})
	})
}
