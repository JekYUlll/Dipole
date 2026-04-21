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

// ListDevices godoc
// @Summary 获取当前用户在线设备列表
// @Tags Session
// @Security BearerAuth
// @Produce json
// @Success 200 {object} DeviceSessionListResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/me/devices [get]
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

// ForceLogoutDevice godoc
// @Summary 下线指定设备
// @Tags Session
// @Security BearerAuth
// @Produce json
// @Param connection_id path string true "连接 ID"
// @Success 200 {object} DeviceLogoutResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/me/devices/{connection_id}/logout [post]
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

// ForceLogoutAll godoc
// @Summary 下线当前用户所有设备
// @Tags Session
// @Security BearerAuth
// @Produce json
// @Success 200 {object} DeviceLogoutResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/me/devices/logout-all [post]
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
