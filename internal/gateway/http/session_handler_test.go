package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

type stubSessionService struct {
	listUserDevicesFn       func(userUUID string) ([]*coresession.DeviceSessionView, error)
	forceLogoutConnectionFn func(userUUID, connectionID string) error
	forceLogoutAllFn        func(userUUID, currentToken string) error
	forceLogoutOtherFn      func(userUUID, currentDeviceID string) error
}

func (s *stubSessionService) ListUserDevices(userUUID string) ([]*coresession.DeviceSessionView, error) {
	return s.listUserDevicesFn(userUUID)
}

func (s *stubSessionService) ForceLogoutConnection(userUUID, connectionID string) error {
	return s.forceLogoutConnectionFn(userUUID, connectionID)
}

func (s *stubSessionService) ForceLogoutAll(userUUID, currentToken string) error {
	return s.forceLogoutAllFn(userUUID, currentToken)
}

func (s *stubSessionService) ForceLogoutOther(userUUID, currentDeviceID string) error {
	return s.forceLogoutOtherFn(userUUID, currentDeviceID)
}

func TestSessionHandlerListDevicesSuccess(t *testing.T) {
	t.Parallel()

	handler := NewSessionHandler(&stubSessionService{
		listUserDevicesFn: func(userUUID string) ([]*coresession.DeviceSessionView, error) {
			if userUUID != "U100" {
				t.Fatalf("unexpected user uuid: %s", userUUID)
			}
			return []*coresession.DeviceSessionView{
				{
					ConnectionID: "C100",
					Device:       "desktop",
					NodeID:       "node-a",
					ConnectedAt:  time.Now().UTC(),
					LastSeenAt:   time.Now().UTC(),
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/me/devices", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDevices(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if string(recorder.Body.Bytes()) == "" {
		t.Fatal("expected device response body")
	}
	for _, forbidden := range []string{"remote_addr", "user_agent", "node_id"} {
		if strings.Contains(string(recorder.Body.Bytes()), forbidden) {
			t.Fatalf("public device projection leaked %s", forbidden)
		}
	}
}

func TestSessionHandlerForceLogoutDeviceNotFound(t *testing.T) {
	t.Parallel()

	handler := NewSessionHandler(&stubSessionService{
		forceLogoutConnectionFn: func(userUUID, connectionID string) error {
			return coresession.ErrSessionNotFound
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/devices/C404/logout", nil)
	context.Params = gin.Params{{Key: "connection_id", Value: "C404"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ForceLogoutDevice(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.SessionNotFound {
		t.Fatalf("expected business code %d, got %v", code.SessionNotFound, response["code"])
	}
}

func TestSessionHandlerForceLogoutAllSuccess(t *testing.T) {
	t.Parallel()

	handler := NewSessionHandler(&stubSessionService{
		forceLogoutAllFn: func(userUUID, currentToken string) error {
			if userUUID != "U100" || currentToken != "TOKEN123" {
				t.Fatalf("unexpected force logout all input: %s %s", userUUID, currentToken)
			}
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/devices/logout-all", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	context.Set(middleware.ContextTokenKey, "TOKEN123")

	handler.ForceLogoutAll(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestSessionHandlerForceLogoutOtherUsesCurrentDeviceID(t *testing.T) {
	t.Parallel()
	handler := NewSessionHandler(&stubSessionService{
		forceLogoutOtherFn: func(userUUID, currentDeviceID string) error {
			if userUUID != "U100" || currentDeviceID != "web-current" {
				t.Fatalf("unexpected logout-other input: %s %s", userUUID, currentDeviceID)
			}
			return nil
		},
	})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/devices/logout-others", nil)
	context.Request.Header.Set("X-Device-ID", "web-current")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	handler.ForceLogoutOther(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestSessionHandlerListDevicesFailure(t *testing.T) {
	t.Parallel()

	handler := NewSessionHandler(&stubSessionService{
		listUserDevicesFn: func(userUUID string) ([]*coresession.DeviceSessionView, error) {
			return nil, errors.New("redis unavailable")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/me/devices", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListDevices(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}
