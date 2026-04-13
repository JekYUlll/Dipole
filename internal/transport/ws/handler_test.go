package ws

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubTokenResolver struct {
	resolveFn func(token string) (string, error)
}

func (s *stubTokenResolver) Resolve(token string) (string, error) {
	if s.resolveFn == nil {
		return "", errors.New("unexpected resolve call")
	}

	return s.resolveFn(token)
}

type stubUserFinder struct {
	getByUUIDFn func(uuid string) (*model.User, error)
}

func (s *stubUserFinder) GetByUUID(uuid string) (*model.User, error) {
	if s.getByUUIDFn == nil {
		return nil, errors.New("unexpected get by uuid call")
	}

	return s.getByUUIDFn(uuid)
}

type stubDirectMessageService struct {
	sendDirectMessageFn func(senderUUID, targetUUID, content string) (*model.Message, error)
}

func (s *stubDirectMessageService) SendDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	if s.sendDirectMessageFn == nil {
		return nil, errors.New("unexpected send direct message call")
	}

	return s.sendDirectMessageFn(senderUUID, targetUUID, content)
}

type stubConversationUpdater struct {
	updateDirectConversationsFn func(message *model.Message) error
}

func (s *stubConversationUpdater) UpdateDirectConversations(message *model.Message) error {
	if s.updateDirectConversationsFn == nil {
		return nil
	}

	return s.updateDirectConversationsFn(message)
}

type wsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type wsEvent struct {
	Type string             `json:"type"`
	Data ConnectedEventData `json:"data"`
}

type chatSentEvent struct {
	Type string       `json:"type"`
	Data ChatSentData `json:"data"`
}

type chatMessageEvent struct {
	Type string          `json:"type"`
	Data ChatMessageData `json:"data"`
}

func TestHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	hub := NewHub()
	userFinder := &stubUserFinder{}
	handler := NewHandler(NewAuthenticator(&stubTokenResolver{}, userFinder), hub, NewDispatcher(hub, &stubDirectMessageService{
		sendDirectMessageFn: func(senderUUID, targetUUID, content string) (*model.Message, error) {
			return nil, errors.New("unexpected send direct message call")
		},
	}, &stubConversationUpdater{}))
	router.GET("/api/v1/ws", handler.Handle)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/api/v1/ws")
	if err != nil {
		t.Fatalf("get ws endpoint: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	var body wsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code.AuthTokenRequired {
		t.Fatalf("expected business code %d, got %d", code.AuthTokenRequired, body.Code)
	}
}

func TestHandlerConnectsAndRegistersClient(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	authenticator := NewAuthenticator(
		&stubTokenResolver{
			resolveFn: func(token string) (string, error) {
				if token != "valid-token" {
					t.Fatalf("unexpected token: %s", token)
				}
				return "U100", nil
			},
		},
		&stubUserFinder{
			getByUUIDFn: func(uuid string) (*model.User, error) {
				if uuid != "U100" {
					t.Fatalf("unexpected user uuid: %s", uuid)
				}

				return &model.User{
					UUID:   "U100",
					Status: model.UserStatusNormal,
				}, nil
			},
		},
	)

	router := gin.New()
	handler := NewHandler(authenticator, hub, NewDispatcher(hub, &stubDirectMessageService{
		sendDirectMessageFn: func(senderUUID, targetUUID, content string) (*model.Message, error) {
			return nil, errors.New("unexpected send direct message call")
		},
	}, &stubConversationUpdater{}))
	router.GET("/api/v1/ws", handler.Handle)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws?token=valid-token"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read connected event: %v", err)
	}

	var evt wsEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		t.Fatalf("unmarshal websocket event: %v", err)
	}
	if evt.Type != TypeConnected {
		t.Fatalf("expected connected event, got %s", evt.Type)
	}
	if evt.Data.UserUUID != "U100" {
		t.Fatalf("expected user uuid U100, got %s", evt.Data.UserUUID)
	}
	if evt.Data.ConnectionCount != 1 {
		t.Fatalf("expected connection count 1, got %d", evt.Data.ConnectionCount)
	}
	if evt.Data.OnlineUserCount != 1 {
		t.Fatalf("expected online user count 1, got %d", evt.Data.OnlineUserCount)
	}
	if hub.UserConnectionCount("U100") != 1 {
		t.Fatalf("expected hub to register 1 connection, got %d", hub.UserConnectionCount("U100"))
	}

	_ = conn.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if hub.UserConnectionCount("U100") == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected hub to unregister client after close, still has %d connections", hub.UserConnectionCount("U100"))
}

func TestHandlerRoutesTextMessageBetweenClients(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	users := map[string]*model.User{
		"U100": {UUID: "U100", IsAdmin: true, Status: model.UserStatusNormal},
		"U200": {UUID: "U200", Status: model.UserStatusNormal},
	}
	authenticator := NewAuthenticator(
		&stubTokenResolver{
			resolveFn: func(token string) (string, error) {
				switch token {
				case "token-a":
					return "U100", nil
				case "token-b":
					return "U200", nil
				default:
					return "", errors.New("unexpected token")
				}
			},
		},
		&stubUserFinder{
			getByUUIDFn: func(uuid string) (*model.User, error) {
				return users[uuid], nil
			},
		},
	)
	dispatcher := NewDispatcher(hub, &stubDirectMessageService{
		sendDirectMessageFn: func(senderUUID, targetUUID, content string) (*model.Message, error) {
			if senderUUID != "U100" {
				t.Fatalf("unexpected sender uuid: %s", senderUUID)
			}
			if targetUUID != "U200" {
				t.Fatalf("unexpected target uuid: %s", targetUUID)
			}
			if content != "hello from U100" {
				t.Fatalf("unexpected content: %s", content)
			}

			return &model.Message{
				UUID:        "M100",
				SenderUUID:  senderUUID,
				TargetUUID:  targetUUID,
				TargetType:  model.MessageTargetDirect,
				MessageType: model.MessageTypeText,
				Content:     content,
				SentAt:      time.Now().UTC(),
			}, nil
		},
	}, &stubConversationUpdater{
		updateDirectConversationsFn: func(message *model.Message) error {
			if message.UUID != "M100" {
				t.Fatalf("unexpected message in conversation updater: %s", message.UUID)
			}
			return nil
		},
	})

	router := gin.New()
	handler := NewHandler(authenticator, hub, dispatcher)
	router.GET("/api/v1/ws", handler.Handle)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	first := dialTestWebSocket(t, server.URL, "token-a")
	second := dialTestWebSocket(t, server.URL, "token-b")
	t.Cleanup(func() { _ = first.Close() })
	t.Cleanup(func() { _ = second.Close() })

	var connectedA wsEvent
	readWebSocketJSON(t, first, &connectedA)
	var connectedB wsEvent
	readWebSocketJSON(t, second, &connectedB)

	payload, err := EncodeCommand(TypeChatSend, SendTextMessageInput{
		TargetUUID: "U200",
		Content:    "hello from U100",
	})
	if err != nil {
		t.Fatalf("encode command: %v", err)
	}
	if err := first.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var ack chatSentEvent
	readWebSocketJSON(t, first, &ack)
	if ack.Type != TypeChatSent {
		t.Fatalf("expected ack type %s, got %s", TypeChatSent, ack.Type)
	}
	if ack.Data.TargetUUID != "U200" {
		t.Fatalf("expected ack target U200, got %s", ack.Data.TargetUUID)
	}
	if !ack.Data.Delivered {
		t.Fatalf("expected delivered ack, got false")
	}

	var incoming chatMessageEvent
	readWebSocketJSON(t, second, &incoming)
	if incoming.Type != TypeChatMessage {
		t.Fatalf("expected incoming type %s, got %s", TypeChatMessage, incoming.Type)
	}
	if incoming.Data.FromUUID != "U100" {
		t.Fatalf("expected from U100, got %s", incoming.Data.FromUUID)
	}
	if incoming.Data.Content != "hello from U100" {
		t.Fatalf("expected message content preserved, got %q", incoming.Data.Content)
	}
}

func TestHandlerRejectsDirectMessageWithoutFriendship(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	users := map[string]*model.User{
		"U100": {UUID: "U100", Status: model.UserStatusNormal},
	}
	authenticator := NewAuthenticator(
		&stubTokenResolver{
			resolveFn: func(token string) (string, error) {
				if token != "token-a" {
					return "", errors.New("unexpected token")
				}
				return "U100", nil
			},
		},
		&stubUserFinder{
			getByUUIDFn: func(uuid string) (*model.User, error) {
				return users[uuid], nil
			},
		},
	)
	dispatcher := NewDispatcher(hub, &stubDirectMessageService{
		sendDirectMessageFn: func(senderUUID, targetUUID, content string) (*model.Message, error) {
			return nil, service.ErrMessageFriendRequired
		},
	}, &stubConversationUpdater{})

	router := gin.New()
	handler := NewHandler(authenticator, hub, dispatcher)
	router.GET("/api/v1/ws", handler.Handle)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	first := dialTestWebSocket(t, server.URL, "token-a")
	t.Cleanup(func() { _ = first.Close() })

	var connected wsEvent
	readWebSocketJSON(t, first, &connected)

	payload, err := EncodeCommand(TypeChatSend, SendTextMessageInput{
		TargetUUID: "U200",
		Content:    "hello from U100",
	})
	if err != nil {
		t.Fatalf("encode command: %v", err)
	}
	if err := first.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var errEvent struct {
		Type string         `json:"type"`
		Data ErrorEventData `json:"data"`
	}
	readWebSocketJSON(t, first, &errEvent)
	if errEvent.Type != TypeError {
		t.Fatalf("expected error event, got %s", errEvent.Type)
	}
	if errEvent.Data.Code != ErrorPermissionDenied {
		t.Fatalf("expected error code %s, got %s", ErrorPermissionDenied, errEvent.Data.Code)
	}
}

func dialTestWebSocket(t *testing.T, serverURL string, token string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/api/v1/ws?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	return conn
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	if err := json.Unmarshal(payload, v); err != nil {
		t.Fatalf("unmarshal websocket payload: %v", err)
	}
}
