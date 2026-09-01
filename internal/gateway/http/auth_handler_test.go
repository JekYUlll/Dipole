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
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

type stubAuthService struct {
	registerFn       func(input coreauth.RegisterInput) (*coreauth.AuthResult, error)
	loginFn          func(input coreauth.LoginInput) (*coreauth.AuthResult, error)
	logoutFn         func(token string) error
	changePasswordFn func(user *model.User, token string, input coreauth.ChangePasswordInput) error
	mcpGrantFn       func(input coreauth.AgentMCPGrantInput) (*coreauth.AgentMCPGrantResult, error)
}

func (s *stubAuthService) IssueAgentMCPGrant(input coreauth.AgentMCPGrantInput) (*coreauth.AgentMCPGrantResult, error) {
	if s.mcpGrantFn == nil {
		return nil, nil
	}
	return s.mcpGrantFn(input)
}

type stubAuthLimiter struct {
	allowRegisterFn func(identifier string) (bool, time.Duration)
	allowLoginFn    func(identifier string) (bool, time.Duration)
}

func (s *stubAuthService) Register(input coreauth.RegisterInput) (*coreauth.AuthResult, error) {
	if s.registerFn == nil {
		return nil, nil
	}
	return s.registerFn(input)
}

func (s *stubAuthService) Login(input coreauth.LoginInput) (*coreauth.AuthResult, error) {
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

func (s *stubAuthService) ChangePassword(user *model.User, token string, input coreauth.ChangePasswordInput) error {
	if s.changePasswordFn == nil {
		return nil
	}
	return s.changePasswordFn(user, token, input)
}

func (s *stubAuthLimiter) AllowRegister(identifier string) (bool, time.Duration) {
	if s.allowRegisterFn == nil {
		return true, 0
	}
	return s.allowRegisterFn(identifier)
}

func (s *stubAuthLimiter) AllowLogin(identifier string) (bool, time.Duration) {
	if s.allowLoginFn == nil {
		return true, 0
	}

	return s.allowLoginFn(identifier)
}

func TestAuthHandlerRegisterSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(&stubAuthService{
		registerFn: func(input coreauth.RegisterInput) (*coreauth.AuthResult, error) {
			if input.Telephone != "13800138000" {
				t.Fatalf("unexpected telephone: %s", input.Telephone)
			}
			return &coreauth.AuthResult{
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

	handler := NewAuthHandler(&stubAuthService{
		registerFn: func(input coreauth.RegisterInput) (*coreauth.AuthResult, error) {
			return nil, coreauth.ErrUserAlreadyExists
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

	handler := NewAuthHandler(&stubAuthService{
		loginFn: func(input coreauth.LoginInput) (*coreauth.AuthResult, error) {
			return nil, coreauth.ErrInvalidCredentials
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

func TestAuthHandlerLoginRateLimited(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(&stubAuthService{
		loginFn: func(input coreauth.LoginInput) (*coreauth.AuthResult, error) {
			t.Fatalf("login service should not be called when rate limited")
			return nil, nil
		},
	}).WithLimiter(&stubAuthLimiter{
		allowLoginFn: func(identifier string) (bool, time.Duration) {
			if identifier != "13800138000" {
				t.Fatalf("unexpected rate limit identifier: %s", identifier)
			}
			return false, 30 * time.Second
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"telephone":"13800138000","password":"badpass"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	handler.Login(context)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.AuthLoginRateLimited {
		t.Fatalf("expected business code %d, got %v", code.AuthLoginRateLimited, response["code"])
	}
}

func TestAuthHandlerLogoutSuccess(t *testing.T) {
	t.Parallel()

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

func TestAuthHandlerChangePasswordUsesAuthenticatedUserAndDisablesCaching(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(&stubAuthService{changePasswordFn: func(user *model.User, token string, input coreauth.ChangePasswordInput) error {
		if user.UUID != "U100" || token != "TOKEN123" || input.CurrentPassword != "old-secret" || input.NewPassword != "new-secret" {
			t.Fatalf("unexpected password change input: user=%+v token=%q input=%+v", user, token, input)
		}
		return nil
	}})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/auth/password", strings.NewReader(`{"current_password":"old-secret","new_password":"new-secret"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	context.Set(middleware.ContextTokenKey, "TOKEN123")

	handler.ChangePassword(context)

	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" || !strings.Contains(recorder.Body.String(), "password changed") {
		t.Fatalf("unexpected password change response: code=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestAuthHandlerChangePasswordDoesNotExposeCurrentPassword(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(&stubAuthService{changePasswordFn: func(*model.User, string, coreauth.ChangePasswordInput) error {
		return coreauth.ErrInvalidCurrentPassword
	}})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/auth/password", strings.NewReader(`{"current_password":"wrong-secret","new_password":"new-secret"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	context.Set(middleware.ContextTokenKey, "TOKEN123")

	handler.ChangePassword(context)

	if recorder.Code != http.StatusBadRequest || strings.Contains(recorder.Body.String(), "wrong-secret") || !strings.Contains(recorder.Body.String(), "current password is invalid") {
		t.Fatalf("unexpected invalid password response: code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthHandlerIssuesAgentMCPGrantFromAuthenticatedPrincipal(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(&stubAuthService{mcpGrantFn: func(input coreauth.AgentMCPGrantInput) (*coreauth.AgentMCPGrantResult, error) {
		if input.UserUUID != "U100" || input.Resource != coreauth.AgentMCPResource || len(input.Scopes) != 1 || input.Scopes[0] != coreauth.AgentMCPReadScope || !input.Consent {
			t.Fatalf("unexpected grant input: %+v", input)
		}
		return &coreauth.AgentMCPGrantResult{
			AccessToken: "MCP_TOKEN", TokenType: "Bearer", ExpiresIn: 900,
			Resource: coreauth.AgentMCPResource, Scope: coreauth.AgentMCPReadScope,
		}, nil
	}})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/agent-mcp/token", strings.NewReader(
		`{"resource":"https://dipole.local/api/v1/agent/mcp","scopes":["dipole.agent.mcp.read"],"consent":true,"user_uuid":"U999"}`,
	))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	handler.IssueAgentMCPGrant(context)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" || !strings.Contains(recorder.Body.String(), `"access_token":"MCP_TOKEN"`) {
		t.Fatalf("unexpected grant response: code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthHandlerRejectsAgentMCPGrantWithoutConsent(t *testing.T) {
	t.Parallel()
	handler := NewAuthHandler(&stubAuthService{mcpGrantFn: func(coreauth.AgentMCPGrantInput) (*coreauth.AgentMCPGrantResult, error) {
		t.Fatal("grant service must not run without consent")
		return nil, nil
	}})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/agent-mcp/token", strings.NewReader(
		`{"resource":"https://dipole.local/api/v1/agent/mcp","scopes":["dipole.agent.mcp.read"],"consent":false}`,
	))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	handler.IssueAgentMCPGrant(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
