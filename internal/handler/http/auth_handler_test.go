package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubAuthService struct {
	registerFn func(input service.RegisterInput) (*service.AuthResult, error)
	loginFn    func(input service.LoginInput) (*service.AuthResult, error)
	logoutFn   func(token string) error
}

func (s *stubAuthService) Register(input service.RegisterInput) (*service.AuthResult, error) {
	if s.registerFn == nil {
		return nil, nil
	}
	return s.registerFn(input)
}

func (s *stubAuthService) Login(input service.LoginInput) (*service.AuthResult, error) {
	if s.loginFn == nil {
		return nil, nil
	}
	return s.loginFn(input)
}

func (s *stubAuthService) Logout(token string) error {
	if s.logoutFn == nil {
		return nil
	}
	return s.logoutFn(token)
}

func TestAuthHandlerRegisterSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(&stubAuthService{
		registerFn: func(input service.RegisterInput) (*service.AuthResult, error) {
			if input.Telephone != "13800138000" {
				t.Fatalf("unexpected telephone: %s", input.Telephone)
			}
			return &service.AuthResult{
				Token: "TOKEN123",
				User: &model.User{
					UUID:      "U100",
					Nickname:  "Alice",
					Telephone: "13800138000",
					Status:    model.UserStatusNormal,
				},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"nickname":"Alice","telephone":"13800138000","password":"123456"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	handler.Register(context)

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

func TestAuthHandlerRegisterConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(&stubAuthService{
		registerFn: func(input service.RegisterInput) (*service.AuthResult, error) {
			return nil, service.ErrUserAlreadyExists
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"nickname":"Alice","telephone":"13800138000","password":"123456"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	handler.Register(context)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
}

func TestAuthHandlerLoginUnauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(&stubAuthService{
		loginFn: func(input service.LoginInput) (*service.AuthResult, error) {
			return nil, service.ErrInvalidCredentials
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"telephone":"13800138000","password":"badpass"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	handler.Login(context)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestAuthHandlerLogoutSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(&stubAuthService{
		logoutFn: func(token string) error {
			if token != "TOKEN123" {
				t.Fatalf("unexpected token: %s", token)
			}
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	context.Set(middleware.ContextTokenKey, "TOKEN123")

	handler.Logout(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestAuthHandlerLogoutFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewAuthHandler(&stubAuthService{
		logoutFn: func(token string) error {
			return errors.New("redis unavailable")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	context.Set(middleware.ContextTokenKey, "TOKEN123")

	handler.Logout(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}
