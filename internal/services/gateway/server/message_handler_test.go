package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
)

type gatewayMessageQueryStub struct {
	directBeforeSeq func(string, string, uint64, int) ([]*model.Message, error)
	groupAfter      func(string, string, uint, int) ([]*model.Message, error)
	offline         func(string, uint, int) ([]*model.Message, error)
}

func (s gatewayMessageQueryStub) ListDirectMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (s gatewayMessageQueryStub) ListDirectMessagesBeforeSeq(user, target string, seq uint64, limit int) ([]*model.Message, error) {
	return s.directBeforeSeq(user, target, seq, limit)
}
func (s gatewayMessageQueryStub) ListDirectMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (s gatewayMessageQueryStub) ListGroupMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
func (s gatewayMessageQueryStub) ListGroupMessagesBeforeSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (s gatewayMessageQueryStub) ListGroupMessagesAfter(user, group string, afterID uint, limit int) ([]*model.Message, error) {
	return s.groupAfter(user, group, afterID, limit)
}
func (s gatewayMessageQueryStub) ListGroupMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}
func (s gatewayMessageQueryStub) ListOfflineMessages(user string, afterID uint, limit int) ([]*model.Message, error) {
	return s.offline(user, afterID, limit)
}

func TestGatewayMessageHandlerPreservesReadCursorRoutes(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		params  gin.Params
		handle  func(*MessageHandler, *gin.Context)
		service gatewayMessageQueryStub
	}{
		{
			name: "direct before seq", path: "/messages/direct/U200?before_seq=41&limit=10", params: gin.Params{{Key: "target_uuid", Value: "U200"}}, handle: (*MessageHandler).ListDirect,
			service: gatewayMessageQueryStub{directBeforeSeq: func(user, target string, seq uint64, limit int) ([]*model.Message, error) {
				if user != "U100" || target != "U200" || seq != 41 || limit != 10 {
					t.Fatalf("unexpected direct request: %q %q %d %d", user, target, seq, limit)
				}
				return []*model.Message{{UUID: "M40", Seq: 40}}, nil
			}},
		},
		{
			name: "group after id", path: "/messages/group/G100?after_id=20&limit=5", params: gin.Params{{Key: "group_uuid", Value: "G100"}}, handle: (*MessageHandler).ListGroup,
			service: gatewayMessageQueryStub{groupAfter: func(user, group string, afterID uint, limit int) ([]*model.Message, error) {
				if user != "U100" || group != "G100" || afterID != 20 || limit != 5 {
					t.Fatalf("unexpected group request: %q %q %d %d", user, group, afterID, limit)
				}
				return []*model.Message{{UUID: "M21"}}, nil
			}},
		},
		{
			name: "offline after id", path: "/messages/offline?after_id=12&limit=3", handle: (*MessageHandler).ListOffline,
			service: gatewayMessageQueryStub{offline: func(user string, afterID uint, limit int) ([]*model.Message, error) {
				if user != "U100" || afterID != 12 || limit != 3 {
					t.Fatalf("unexpected offline request: %q %d %d", user, afterID, limit)
				}
				return []*model.Message{{UUID: "M13"}}, nil
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, test.path, nil)
			context.Params = test.params
			context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
			test.handle(NewMessageHandler(test.service), context)
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", recorder.Code)
			}
		})
	}
}

func TestGatewayMessageHandlerRejectsMixedCursorDomains(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/messages/direct/U200?before_id=1&before_seq=2", nil)
	context.Params = gin.Params{{Key: "target_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	NewMessageHandler(gatewayMessageQueryStub{}).ListDirect(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}
