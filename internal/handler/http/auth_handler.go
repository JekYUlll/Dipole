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
	"github.com/JekYUlll/Dipole/internal/service"
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
	Register(input service.RegisterInput) (*service.AuthResult, error)
	Login(input service.LoginInput) (*service.AuthResult, error)
	Logout(token string) error
}

func NewAuthHandler(service authService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) WithLimiter(limiter authRateLimiter) *AuthHandler {
	h.limiter = limiter
	return h
}

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
		case errors.Is(err, service.ErrInvalidTelephone):
			ErrorWithCode(c, http.StatusBadRequest, code.AuthInvalidTelephone, "telephone format is invalid")
		case errors.Is(err, service.ErrUserAlreadyExists):
			ErrorWithCode(c, http.StatusConflict, code.AuthUserAlreadyExists, "telephone already registered")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.NewAuthResponse(result))
}

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
		case errors.Is(err, service.ErrInvalidCredentials):
			ErrorWithCode(c, http.StatusUnauthorized, code.AuthInvalidCredentials, "telephone or password is invalid")
		case errors.Is(err, service.ErrUserDisabled):
			ErrorWithCode(c, http.StatusForbidden, code.AuthUserDisabled, "user is disabled")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.NewAuthResponse(result))
}

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
