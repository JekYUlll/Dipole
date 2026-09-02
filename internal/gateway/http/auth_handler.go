package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

type AuthHandler struct {
	service authService
	limiter authRateLimiter
}

type authRateLimiter interface {
	AllowRegister(identifier string) (bool, time.Duration)
	AllowLogin(identifier string) (bool, time.Duration)
}

type authService interface {
	Register(input coreauth.RegisterInput) (*coreauth.AuthResult, error)
	Login(input coreauth.LoginInput) (*coreauth.AuthResult, error)
	Logout(token string) error
	ChangePassword(user *model.User, token string, input coreauth.ChangePasswordInput) error
	IssueAgentMCPGrant(input coreauth.AgentMCPGrantInput) (*coreauth.AgentMCPGrantResult, error)
}

func NewAuthHandler(service authService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) WithLimiter(limiter authRateLimiter) *AuthHandler {
	h.limiter = limiter
	return h
}

// Register godoc
// @Summary 用户注册
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body httpdto.RegisterRequest true "注册信息"
// @Success 200 {object} AuthResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 409 {object} ErrorEnvelope
// @Failure 429 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var request httpdto.RegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	if h.limiter != nil {
		identifier := strings.TrimSpace(request.Telephone)
		if identifier == "" {
			identifier = c.ClientIP()
		}
		allowed, retryAfter := h.limiter.AllowRegister(identifier)
		if !allowed {
			ErrorWithCode(
				c,
				http.StatusTooManyRequests,
				code.AuthLoginRateLimited,
				formatRetryAfterMessage("too many register attempts", retryAfter),
			)
			return
		}
	}

	result, err := h.service.Register(request.ToInput())
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrInvalidTelephone):
			ErrorWithCode(c, http.StatusBadRequest, code.AuthInvalidTelephone, "telephone format is invalid")
		case errors.Is(err, coreauth.ErrUserAlreadyExists):
			ErrorWithCode(c, http.StatusConflict, code.AuthUserAlreadyExists, "telephone already registered")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.NewAuthResponse(result))
}

// Login godoc
// @Summary 用户登录
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body httpdto.LoginRequest true "登录信息"
// @Success 200 {object} AuthResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 429 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var request httpdto.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	if h.limiter != nil {
		identifier := strings.TrimSpace(request.Telephone)
		if identifier == "" {
			identifier = c.ClientIP()
		}
		allowed, retryAfter := h.limiter.AllowLogin(identifier)
		if !allowed {
			ErrorWithCode(
				c,
				http.StatusTooManyRequests,
				code.AuthLoginRateLimited,
				formatRetryAfterMessage("too many login attempts", retryAfter),
			)
			return
		}
	}

	result, err := h.service.Login(request.ToInput())
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrInvalidCredentials):
			ErrorWithCode(c, http.StatusUnauthorized, code.AuthInvalidCredentials, "telephone or password is invalid")
		case errors.Is(err, coreauth.ErrUserDisabled):
			ErrorWithCode(c, http.StatusForbidden, code.AuthUserDisabled, "user is disabled")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.NewAuthResponse(result))
}

// Logout godoc
// @Summary 用户登出
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MessageOnlyResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	token, ok := middleware.CurrentToken(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.Logout(token); err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.AuthLogoutFailed, err.Error())
		return
	}

	Success(c, gin.H{
		"message": "logout success",
	})
}

// ChangePassword godoc
// @Summary 修改当前用户密码
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body httpdto.ChangePasswordRequest true "当前密码与新密码"
// @Success 200 {object} MessageOnlyResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /auth/password [patch]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authenticated principal is required")
		return
	}
	token, ok := middleware.CurrentToken(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	var request httpdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "current_password and new_password must be 6 to 32 characters")
		return
	}
	if err := h.service.ChangePassword(user, token, request.ToInput()); err != nil {
		switch {
		case errors.Is(err, coreauth.ErrInvalidCurrentPassword):
			ErrorWithCode(c, http.StatusBadRequest, code.AuthCurrentPasswordInvalid, "current password is invalid")
		case errors.Is(err, coreauth.ErrPasswordUnchanged):
			ErrorWithCode(c, http.StatusBadRequest, code.AuthPasswordUnchanged, "new password must differ from current password")
		case errors.Is(err, coreauth.ErrInvalidPassword):
			ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "current_password and new_password must be 6 to 32 characters")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, "password change failed")
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	Success(c, gin.H{"message": "password changed, please sign in again"})
}

// IssueAgentMCPGrant godoc
// @Summary 签发第一方 Agent MCP 短期访问令牌
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body httpdto.AgentMCPGrantRequest true "MCP resource、scope 与显式授权"
// @Success 200 {object} SuccessEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /auth/agent-mcp/token [post]
func (h *AuthHandler) IssueAgentMCPGrant(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authenticated principal is required")
		return
	}
	var request httpdto.AgentMCPGrantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}
	result, err := h.service.IssueAgentMCPGrant(coreauth.AgentMCPGrantInput{
		UserUUID: user.UUID, Resource: request.Resource, Scopes: request.Scopes, Consent: request.Consent,
	})
	if err != nil {
		if errors.Is(err, coreauth.ErrInvalidAgentMCPGrant) {
			ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "Agent MCP resource, scope, and consent must match the supported grant")
			return
		}
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	Success(c, httpdto.NewAgentMCPGrantResponse(result))
}

func formatRetryAfterMessage(message string, retryAfter time.Duration) string {
	seconds := int(retryAfter.Seconds())
	if retryAfter > 0 && seconds == 0 {
		seconds = 1
	}
	if seconds <= 0 {
		return message
	}

	return fmt.Sprintf("%s, retry after %d seconds", message, seconds)
}
