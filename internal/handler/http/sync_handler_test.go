package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubSyncService struct {
	userUUID string
	afterSeq uint64
	limit    int
}

func (s *stubSyncService) List(userUUID string, afterSeq uint64, limit int) (*applicationPort.SyncPage, error) {
	s.userUUID = userUUID
	s.afterSeq = afterSeq
	s.limit = limit
	return &applicationPort.SyncPage{
		Items: []*model.SyncMessage{{
			SyncSeq:         8,
			ConversationKey: "direct:U100:U200",
			Message:         &model.Message{UUID: "M8"},
		}},
		NextSeq: 8,
	}, nil
}

func TestSyncHandlerListSuccess(t *testing.T) {
	stub := &stubSyncService{}
	handler := NewSyncHandler(stub)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sync?after_seq=7&limit=20", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U200"})

	handler.List(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if stub.userUUID != "U200" || stub.afterSeq != 7 || stub.limit != 20 {
		t.Fatalf("unexpected sync query: %+v", stub)
	}
	var response struct {
		Data struct {
			NextSeq uint64 `json:"next_seq"`
			Items   []struct {
				SyncSeq uint64 `json:"sync_seq"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Data.NextSeq != 8 || len(response.Data.Items) != 1 || response.Data.Items[0].SyncSeq != 8 {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestSyncHandlerRejectsInvalidCursor(t *testing.T) {
	handler := NewSyncHandler(&stubSyncService{})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sync?after_seq=bad", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U200"})

	handler.List(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}
