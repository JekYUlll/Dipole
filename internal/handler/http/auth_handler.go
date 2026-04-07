package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

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
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Register(input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTelephone):
			Error(c, http.StatusBadRequest, "telephone format is invalid")
		case errors.Is(err, service.ErrUserAlreadyExists):
			Error(c, http.StatusConflict, "telephone already registered")
		default:
			Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	Success(c, result)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Login(input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			Error(c, http.StatusUnauthorized, "telephone or password is invalid")
		case errors.Is(err, service.ErrUserDisabled):
			Error(c, http.StatusForbidden, "user is disabled")
		default:
			Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	Success(c, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, ok := middleware.CurrentToken(c)
	if !ok {
		Error(c, http.StatusUnauthorized, "authorization token is required")
		return
	}

	if err := h.service.Logout(token); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	Success(c, gin.H{
		"message": "logout success",
	})
}
