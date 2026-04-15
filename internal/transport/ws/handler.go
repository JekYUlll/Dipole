package ws

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/logger"
)

type Handler struct {
	authenticator *Authenticator
	hub           *Hub
	dispatcher    inboundHandler
	upgrader      websocket.Upgrader
}

func NewHandler(authenticator *Authenticator, hub *Hub, dispatcher inboundHandler) *Handler {
	return &Handler{
		authenticator: authenticator,
		hub:           hub,
		dispatcher:    dispatcher,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) Handle(c *gin.Context) {
	sessionUser, token, err := h.authenticator.Authenticate(c.Request)
	if err != nil {
		h.writeAuthError(c, err)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warn("websocket upgrade failed", zap.Error(err))
		return
	}

	client := NewClient(h.hub, conn, sessionUser, token, extractConnectionIdentity(c.Request), h.dispatcher)
	h.hub.Register(client)

	if err := client.SendEvent(TypeConnected, ConnectedEventData{
		UserUUID:        sessionUser.UUID,
		ConnectionCount: h.hub.UserConnectionCount(sessionUser.UUID),
		OnlineUserCount: h.hub.OnlineUserCount(),
	}); err != nil {
		logger.Warn("enqueue websocket connected event failed",
			zap.String("user_uuid", sessionUser.UUID),
			zap.Error(err),
		)
		h.hub.Unregister(client)
		return
	}

	client.Run()
}

func extractConnectionIdentity(r *http.Request) ConnectionIdentity {
	if r == nil {
		return ConnectionIdentity{}
	}

	device := strings.TrimSpace(r.URL.Query().Get("device"))
	if device == "" {
		device = strings.TrimSpace(r.Header.Get("X-Device"))
	}
	if device == "" {
		device = detectDeviceFromUserAgent(r.UserAgent())
	}

	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		deviceID = strings.TrimSpace(r.Header.Get("X-Device-ID"))
	}

	return ConnectionIdentity{
		Device:     device,
		DeviceID:   deviceID,
		UserAgent:  strings.TrimSpace(r.UserAgent()),
		RemoteAddr: strings.TrimSpace(r.RemoteAddr),
	}
}

func detectDeviceFromUserAgent(userAgent string) string {
	normalized := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case strings.Contains(normalized, "android"):
		return "android"
	case strings.Contains(normalized, "iphone"), strings.Contains(normalized, "ipad"), strings.Contains(normalized, "ios"):
		return "ios"
	case strings.Contains(normalized, "windows"), strings.Contains(normalized, "macintosh"), strings.Contains(normalized, "linux"):
		return "desktop"
	case strings.Contains(normalized, "mozilla"), strings.Contains(normalized, "chrome"), strings.Contains(normalized, "safari"):
		return "web"
	default:
		return "unknown"
	}
}

func (h *Handler) writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTokenRequired):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    code.AuthTokenRequired,
			"message": ErrTokenRequired.Error(),
		})
	case errors.Is(err, ErrTokenInvalid), errors.Is(err, ErrUserSessionInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    code.AuthTokenInvalid,
			"message": err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    code.Internal,
			"message": err.Error(),
		})
	}
}
