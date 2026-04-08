package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	result, err := h.service.Register(input)
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

	Success(c, newAuthResponse(result))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	result, err := h.service.Login(input)
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

	Success(c, newAuthResponse(result))
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
