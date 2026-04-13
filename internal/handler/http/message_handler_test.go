package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubMessageService struct {
	listDirectFn func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	listGroupFn  func(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
}

func (s *stubMessageService) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	if s.listDirectFn == nil {
		return nil, nil
	}

	return s.listDirectFn(currentUserUUID, targetUUID, beforeID, limit)
}

func (s *stubMessageService) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	if s.listGroupFn == nil {
		return nil, nil
	}

	return s.listGroupFn(currentUserUUID, groupUUID, beforeID, limit)
}

func TestMessageHandlerListDirectSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listDirectFn: func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			if currentUserUUID != "U100" {
				t.Fatalf("unexpected current user uuid: %s", currentUserUUID)
			}
			if targetUUID != "U200" {
				t.Fatalf("unexpected target uuid: %s", targetUUID)
			}
			if beforeID != 20 {
				t.Fatalf("unexpected before_id: %d", beforeID)
			}
			if limit != 10 {
				t.Fatalf("unexpected limit: %d", limit)
			}

			return []*model.Message{
				{
					ID:          21,
					UUID:        "M21",
					SenderUUID:  "U100",
					TargetUUID:  "U200",
					TargetType:  model.MessageTargetDirect,
					MessageType: model.MessageTypeText,
					Content:     "hello",
					SentAt:      time.Now().UTC(),
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U200?before_id=20&limit=10", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.Success {
		t.Fatalf("expected business code %d, got %v", code.Success, response["code"])
	}
}

func TestMessageHandlerListDirectRejectsInvalidBeforeID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listDirectFn: func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			return nil, errors.New("should not be called")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U200?before_id=abc", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestMessageHandlerListDirectNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listDirectFn: func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			return nil, service.ErrMessageTargetNotFound
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U404", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U404"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.MessageTargetNotFound {
		t.Fatalf("expected business code %d, got %v", code.MessageTargetNotFound, response["code"])
	}
}

func TestMessageHandlerListDirectRequiresFriendship(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listDirectFn: func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			return nil, service.ErrMessageFriendRequired
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U200", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestMessageHandlerListGroupSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listGroupFn: func(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			if currentUserUUID != "U100" || groupUUID != "G100" {
				t.Fatalf("unexpected group query: %s %s", currentUserUUID, groupUUID)
			}
			return []*model.Message{
				{
					ID:          30,
					UUID:        "M30",
					SenderUUID:  "U100",
					TargetUUID:  "G100",
					TargetType:  model.MessageTargetGroup,
					MessageType: model.MessageTypeText,
					Content:     "hello group",
					SentAt:      time.Now().UTC(),
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/group/G100", nil)
	context.Params = gin.Params{{Key: "group_uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListGroup(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestMessageHandlerListGroupForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewMessageHandler(&stubMessageService{
		listGroupFn: func(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
			return nil, service.ErrMessageGroupForbidden
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/group/G100", nil)
	context.Params = gin.Params{{Key: "group_uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListGroup(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}
