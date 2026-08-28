package logger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRedactQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		query string
	}{
		{name: "ordinary query", raw: "group_id=G1&before_seq=10", query: "before_seq=10&group_id=G1"},
		{name: "credential keys and duplicate values", raw: "token=one&%61ccess_token=two&token=three&device=web", query: "access_token=REDACTED&device=web&token=REDACTED&token=REDACTED"},
		{name: "case insensitive key", raw: "ToKeN=secret&device_id=D1", query: "ToKeN=REDACTED&device_id=D1"},
		{name: "credential suffixes", raw: "X-Amz-Signature=signed&session-token=session&cursor=next", query: "X-Amz-Signature=REDACTED&cursor=next&session-token=REDACTED"},
		{name: "malformed query", raw: "token=secret;device=web", query: "REDACTED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := redactQuery(test.raw); got != test.query {
				t.Fatalf("redactQuery() = %q, want %q", got, test.query)
			}
		})
	}
}

func TestGinLoggerRedactsWebSocketCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(ginLogger(zap.New(core)))
	router.GET("/api/v1/ws", func(c *gin.Context) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		_ = connection.Close()
	})

	server := httptest.NewServer(router)
	defer server.Close()
	header := http.Header{"Authorization": []string{"Bearer header-secret"}}
	connection, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/ws?token=query-secret&%61ccess_token=encoded-secret&device=web",
		header,
	)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	_ = connection.Close()

	deadline := time.Now().Add(time.Second)
	for logs.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["path"] != "/api/v1/ws?access_token=REDACTED&device=web&token=REDACTED" {
		t.Fatalf("logged path = %q", fields["path"])
	}
	if entries[0].Message != "request completed" {
		t.Fatalf("logged message = %q", entries[0].Message)
	}
	for _, field := range fields {
		if field == "query-secret" || field == "Bearer header-secret" {
			t.Fatalf("access log leaked a credential: %v", field)
		}
	}
}
