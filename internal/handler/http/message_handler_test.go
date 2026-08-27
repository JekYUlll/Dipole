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
	listDirectFn          func(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	listDirectBeforeSeqFn func(currentUserUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error)
	listDirectAfterSeqFn  func(currentUserUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error)
	listGroupFn           func(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	listGroupBeforeSeqFn  func(currentUserUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error)
	listGroupAfterFn      func(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error)
	listGroupAfterSeqFn   func(currentUserUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error)
	listOfflineFn         func(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error)
}

func (s *stubMessageService) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	if s.listDirectFn == nil {
		return nil, nil
	}

	return s.listDirectFn(currentUserUUID, targetUUID, beforeID, limit)
}

func (s *stubMessageService) ListDirectMessagesBeforeSeq(currentUserUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	if s.listDirectBeforeSeqFn == nil {
		return nil, nil
	}
	return s.listDirectBeforeSeqFn(currentUserUUID, targetUUID, beforeSeq, limit)
}

func (s *stubMessageService) ListDirectMessagesAfterSeq(currentUserUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	if s.listDirectAfterSeqFn == nil {
		return nil, nil
	}
	return s.listDirectAfterSeqFn(currentUserUUID, targetUUID, afterSeq, limit)
}

func (s *stubMessageService) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	if s.listGroupFn == nil {
		return nil, nil
	}

	return s.listGroupFn(currentUserUUID, groupUUID, beforeID, limit)
}

func (s *stubMessageService) ListGroupMessagesBeforeSeq(currentUserUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	if s.listGroupBeforeSeqFn == nil {
		return nil, nil
	}
	return s.listGroupBeforeSeqFn(currentUserUUID, groupUUID, beforeSeq, limit)
}

func TestMessageHandlerListDirectBeforeSeq(t *testing.T) {
	called := false
	handler := NewMessageHandler(&stubMessageService{
		listDirectBeforeSeqFn: func(userUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
			called = true
			if userUUID != "U100" || targetUUID != "U200" || beforeSeq != 41 || limit != 10 {
				t.Fatalf("unexpected Seq query: user=%q target=%q before=%d limit=%d", userUUID, targetUUID, beforeSeq, limit)
			}
			return []*model.Message{{Seq: 40, UUID: "M40"}}, nil
		},
	})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U200?before_seq=41&limit=10", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)
	if recorder.Code != http.StatusOK || !called {
		t.Fatalf("expected Seq route status=200 called=true, got status=%d called=%t", recorder.Code, called)
	}
}

func TestMessageHandlerListDirectAfterSeq(t *testing.T) {
	called := false
	handler := NewMessageHandler(&stubMessageService{
		listDirectAfterSeqFn: func(userUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
			called = true
			if userUUID != "U100" || targetUUID != "U200" || afterSeq != 41 || limit != 10 {
				t.Fatalf("unexpected Seq query: user=%q target=%q after=%d limit=%d", userUUID, targetUUID, afterSeq, limit)
			}
			return []*model.Message{{Seq: 42, UUID: "M42"}}, nil
		},
	})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/direct/U200?after_seq=41&limit=10", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDirect(context)
	if recorder.Code != http.StatusOK || !called {
		t.Fatalf("expected Seq route status=200 called=true, got status=%d called=%t", recorder.Code, called)
	}
}

func TestMessageHandlerRejectsMixedHistoryCursorDomains(t *testing.T) {
	for _, test := range []struct {
		name   string
		path   string
		group  bool
		params gin.Params
	}{
		{name: "direct zero ID plus Seq", path: "/api/v1/messages/direct/U200?before_id=0&before_seq=41", params: gin.Params{{Key: "target_uuid", Value: "U200"}}},
		{name: "direct two Seq directions", path: "/api/v1/messages/direct/U200?before_seq=41&after_seq=40", params: gin.Params{{Key: "target_uuid", Value: "U200"}}},
		{name: "group ID plus Seq", path: "/api/v1/messages/group/G100?before_id=20&before_seq=41", group: true, params: gin.Params{{Key: "group_uuid", Value: "G100"}}},
		{name: "group two Seq directions", path: "/api/v1/messages/group/G100?before_seq=41&after_seq=40", group: true, params: gin.Params{{Key: "group_uuid", Value: "G100"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, test.path, nil)
			context.Params = test.params
			context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
			handler := NewMessageHandler(&stubMessageService{})
			if test.group {
				handler.ListGroup(context)
			} else {
				handler.ListDirect(context)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", recorder.Code)
			}
		})
	}
}

func (s *stubMessageService) ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	if s.listGroupAfterFn == nil {
		return nil, nil
	}

	return s.listGroupAfterFn(currentUserUUID, groupUUID, afterID, limit)
}

func (s *stubMessageService) ListGroupMessagesAfterSeq(currentUserUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	if s.listGroupAfterSeqFn == nil {
		return nil, nil
	}
	return s.listGroupAfterSeqFn(currentUserUUID, groupUUID, afterSeq, limit)
}

func (s *stubMessageService) ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error) {
	if s.listOfflineFn == nil {
		return nil, nil
	}

	return s.listOfflineFn(currentUserUUID, afterID, limit)
}

func TestMessageHandlerListDirectSuccess(t *testing.T) {
	t.Parallel()

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

func TestMessageHandlerListGroupAfterSuccess(t *testing.T) {
	t.Parallel()

	handler := NewMessageHandler(&stubMessageService{
		listGroupAfterFn: func(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
			if currentUserUUID != "U100" || groupUUID != "G100" {
				t.Fatalf("unexpected group query: %s %s", currentUserUUID, groupUUID)
			}
			if afterID != 30 {
				t.Fatalf("unexpected after id: %d", afterID)
			}
			if limit != 15 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []*model.Message{
				{
					ID:          31,
					UUID:        "M31",
					SenderUUID:  "U200",
					TargetUUID:  "G100",
					TargetType:  model.MessageTargetGroup,
					MessageType: model.MessageTypeText,
					Content:     "new group message",
					SentAt:      time.Now().UTC(),
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/group/G100?after_id=30&limit=15", nil)
	context.Params = gin.Params{{Key: "group_uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListGroup(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestMessageHandlerListGroupAfterSequenceFromZero(t *testing.T) {
	called := false
	handler := NewMessageHandler(&stubMessageService{listGroupAfterSeqFn: func(userUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
		called = true
		if userUUID != "U100" || groupUUID != "G100" || afterSeq != 0 || limit != 20 {
			t.Fatalf("unexpected sequence query: user=%s group=%s after=%d limit=%d", userUUID, groupUUID, afterSeq, limit)
		}
		return []*model.Message{{UUID: "M1", Seq: 1}}, nil
	}})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/group/G100?after_seq=0&limit=20", nil)
	context.Params = gin.Params{{Key: "group_uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	handler.ListGroup(context)
	if recorder.Code != http.StatusOK || !called {
		t.Fatalf("expected sequence path, status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMessageHandlerListOfflineSuccess(t *testing.T) {
	t.Parallel()

	handler := NewMessageHandler(&stubMessageService{
		listOfflineFn: func(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error) {
			if currentUserUUID != "U100" {
				t.Fatalf("unexpected current user uuid: %s", currentUserUUID)
			}
			if afterID != 30 {
				t.Fatalf("unexpected after_id: %d", afterID)
			}
			if limit != 15 {
				t.Fatalf("unexpected limit: %d", limit)
			}

			return []*model.Message{
				{
					ID:          31,
					UUID:        "M31",
					SenderUUID:  "U200",
					TargetUUID:  "U100",
					TargetType:  model.MessageTargetDirect,
					MessageType: model.MessageTypeText,
					Content:     "offline hello",
					SentAt:      time.Now().UTC(),
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/offline?after_id=30&limit=15", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListOffline(context)

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

func TestMessageHandlerListOfflineRejectsInvalidAfterID(t *testing.T) {
	t.Parallel()

	handler := NewMessageHandler(&stubMessageService{
		listOfflineFn: func(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error) {
			return nil, errors.New("should not be called")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/messages/offline?after_id=bad", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListOffline(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}
