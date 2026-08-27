package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"github.com/gin-gonic/gin"
)

func TestCorrelationPropagatesRequestContextAndResponseHeaders(t *testing.T) {
	engine := gin.New()
	engine.Use(Correlation())
	engine.GET("/", func(c *gin.Context) {
		ids := correlation.FromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"request_id": ids.RequestID, "trace_id": ids.TraceID})
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(correlation.RequestHeader, "request-1")
	request.Header.Set(correlation.TraceHeader, "trace-1")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Header().Get(correlation.RequestHeader) != "request-1" || response.Header().Get(correlation.TraceHeader) != "trace-1" {
		t.Fatalf("unexpected response headers: %v", response.Header())
	}
}
