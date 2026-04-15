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

type stubConversationService struct {
	listForUserFn    func(userUUID string, limit int) ([]*service.ConversationView, error)
	markDirectReadFn func(userUUID, targetUUID string) (*service.ConversationReadReceipt, error)
	markGroupReadFn  func(userUUID, groupUUID string) error
}

func (s *stubConversationService) ListForUser(userUUID string, limit int) ([]*service.ConversationView, error) {
	if s.listForUserFn == nil {
		return nil, nil
	}

	return s.listForUserFn(userUUID, limit)
}

func (s *stubConversationService) MarkDirectConversationRead(userUUID, targetUUID string) (*service.ConversationReadReceipt, error) {
	if s.markDirectReadFn == nil {
		return nil, nil
	}

	return s.markDirectReadFn(userUUID, targetUUID)
}

func (s *stubConversationService) MarkGroupConversationRead(userUUID, groupUUID string) error {
	if s.markGroupReadFn == nil {
		return nil
	}

	return s.markGroupReadFn(userUUID, groupUUID)
}

func TestConversationHandlerListSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewConversationHandler(&stubConversationService{
		listForUserFn: func(userUUID string, limit int) ([]*service.ConversationView, error) {
			if userUUID != "U100" {
				t.Fatalf("unexpected user uuid: %s", userUUID)
			}
			if limit != 10 {
				t.Fatalf("unexpected limit: %d", limit)
			}

			return []*service.ConversationView{
				{
					Conversation: &model.Conversation{
						UserUUID:           "U100",
						TargetType:         model.MessageTargetDirect,
						TargetUUID:         "U200",
						ConversationKey:    model.DirectConversationKey("U100", "U200"),
						LastMessageUUID:    "M100",
						LastMessageType:    model.MessageTypeText,
						LastMessagePreview: "hello",
						LastMessageAt:      time.Now().UTC(),
						UnreadCount:        1,
					},
					TargetUser: &model.User{
						UUID:     "U200",
						Nickname: "Alice",
						Avatar:   "avatar",
						Status:   model.UserStatusNormal,
					},
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/conversations?limit=10", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.List(context)

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

func TestConversationHandlerMarkDirectReadNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewConversationHandler(&stubConversationService{
		markDirectReadFn: func(userUUID, targetUUID string) (*service.ConversationReadReceipt, error) {
			return nil, service.ErrConversationTargetNotFound
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/direct/U404/read", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U404"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.MarkDirectRead(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.ConversationTargetNotFound {
		t.Fatalf("expected business code %d, got %v", code.ConversationTargetNotFound, response["code"])
	}
}

func TestConversationHandlerMarkDirectReadInternal(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewConversationHandler(&stubConversationService{
		markDirectReadFn: func(userUUID, targetUUID string) (*service.ConversationReadReceipt, error) {
			return nil, errors.New("boom")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/direct/U200/read", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.MarkDirectRead(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}

func TestConversationHandlerMarkGroupReadForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewConversationHandler(&stubConversationService{
		markGroupReadFn: func(userUUID, groupUUID string) error {
			return service.ErrConversationPermissionDenied
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/group/G100/read", nil)
	context.Params = gin.Params{{Key: "group_uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.MarkGroupRead(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}
