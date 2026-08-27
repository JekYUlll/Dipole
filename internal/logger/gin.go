package logger

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		if rawQuery != "" {
			path = path + "?" + rawQuery
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

		httpLogger := L().Named("http")
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
