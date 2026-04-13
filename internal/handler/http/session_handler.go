package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type sessionService interface {
	ListUserDevices(userUUID string) ([]*service.DeviceSessionView, error)
	ForceLogoutConnection(userUUID, connectionID string) error
	ForceLogoutAll(userUUID, currentToken string) error
}

type SessionHandler struct {
	service sessionService
}

func NewSessionHandler(service sessionService) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) ListDevices(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	devices, err := h.service.ListUserDevices(currentUser.UUID)
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.ToDeviceSessionResponses(devices))
}

func (h *SessionHandler) ForceLogoutDevice(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.ForceLogoutConnection(currentUser.UUID, c.Param("connection_id")); err != nil {
		switch {
		case errors.Is(err, service.ErrSessionConnectionRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.SessionConnectionRequired, "connection_id is required")
		case errors.Is(err, service.ErrSessionNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.SessionNotFound, "session not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"message":       "device session logged out",
		"connection_id": c.Param("connection_id"),
	})
}

func (h *SessionHandler) ForceLogoutAll(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	currentToken, _ := middleware.CurrentToken(c)
	if err := h.service.ForceLogoutAll(currentUser.UUID, currentToken); err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, gin.H{
		"message": "all device sessions logged out",
	})
}
