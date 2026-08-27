package middleware

import (
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"github.com/gin-gonic/gin"
)

func Correlation() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, ids := correlation.Ensure(
			c.Request.Context(),
			c.GetHeader(correlation.RequestHeader),
			c.GetHeader(correlation.TraceHeader),
		)
		c.Request = c.Request.WithContext(ctx)
		c.Header(correlation.RequestHeader, ids.RequestID)
		c.Header(correlation.TraceHeader, ids.TraceID)
		c.Next()
	}
}
