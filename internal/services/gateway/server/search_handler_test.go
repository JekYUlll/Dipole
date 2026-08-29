package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
)

type searchHandlerStub struct {
	principal string
	text      string
	limit     int
	documents []*model.MessageSearchDocument
	err       error
}

func (s *searchHandlerStub) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	s.principal, s.text, s.limit = principal, text, limit
	return s.documents, s.err
}

func TestSearchHandlerReturnsProjectionResponse(t *testing.T) {
	sentAt := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)
	service := &searchHandlerStub{documents: []*model.MessageSearchDocument{nil, {
		MessageUUID: "M1", ConversationKey: "group:G1", MessageSeq: 9, Revision: 2,
		SenderUUID: "U2", MessageType: model.MessageTypeText, Content: "migration", SentAt: sentAt,
	}}}
	handler := NewSearchHandler(service)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/search?q=%20migration%20&limit=25", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})

	handler.Search(context)

	if recorder.Code != http.StatusOK || service.principal != "U1" || service.text != "migration" || service.limit != 25 {
		t.Fatalf("Search response: code=%d principal=%q text=%q limit=%d body=%s", recorder.Code, service.principal, service.text, service.limit, recorder.Body.String())
	}
	for _, expected := range []string{`"message_id":"M1"`, `"conversation_key":"group:G1"`, `"message_seq":9`, `"revision":2`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("Search response missing %s: %s", expected, recorder.Body.String())
		}
	}
}

func TestSearchHandlerValidatesQuery(t *testing.T) {
	for _, path := range []string{
		"/api/v1/messages/search",
		"/api/v1/messages/search?q=" + strings.Repeat("x", 257),
		"/api/v1/messages/search?q=migration&limit=0",
		"/api/v1/messages/search?q=migration&limit=101",
		"/api/v1/messages/search?q=migration&limit=bad",
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, path, nil)
		context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})
		NewSearchHandler(&searchHandlerStub{}).Search(context)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid query %s to fail, got %d", path, recorder.Code)
		}
	}
}

func TestSearchHandlerBoundsDependencyFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/search?q=migration", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})
	NewSearchHandler(&searchHandlerStub{err: errors.New("Elasticsearch secret detail")}).Search(context)
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), "secret detail") {
		t.Fatalf("expected bounded Search dependency failure, got %d %s", recorder.Code, recorder.Body.String())
	}
}
