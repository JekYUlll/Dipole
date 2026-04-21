package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/logger"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = pongWait * 9 / 10
	maxMessageSize = 4 << 10
	sendBufferSize = 16
)

var ErrSendQueueFull = errors.New("websocket send queue is full")
var ErrClientClosed = errors.New("websocket client is closed")

type Client struct {
	hub            *Hub
	conn           *websocket.Conn
	sessionUser    *SessionUser
	token          string
	connectionID   string
	identity       ConnectionIdentity
	connectedAt    time.Time
	send           chan []byte
	messageHandler inboundHandler
	closeOnce      sync.Once
	closed         atomic.Bool
	log            *zap.Logger
}

func NewClient(hub *Hub, conn *websocket.Conn, sessionUser *SessionUser, token string, identity ConnectionIdentity, messageHandler inboundHandler) *Client {
	connectedAt := time.Now().UTC()
	clientLogger := logger.Named("ws").With(
		zap.String("user_uuid", sessionUser.UUID),
		zap.String("remote_addr", conn.RemoteAddr().String()),
	)

	return &Client{
		hub:            hub,
		conn:           conn,
		sessionUser:    sessionUser,
		token:          token,
		connectionID:   newConnectionID(),
		identity:       identity,
		connectedAt:    connectedAt,
		send:           make(chan []byte, sendBufferSize),
		messageHandler: messageHandler,
		log:            clientLogger,
	}
}

func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

func (c *Client) EnqueueJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal websocket payload: %w", err)
	}

	return c.Enqueue(payload)
}

func (c *Client) Enqueue(payload []byte) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	select {
	case c.send <- payload:
		return nil
	default:
		return ErrSendQueueFull
	}
}

func (c *Client) SendEvent(eventType string, data any) error {
	return c.EnqueueJSON(OutboundEvent{
		Type: eventType,
		Data: data,
	})
}

func (c *Client) SendError(code, message, requestType, clientMessageID string) error {
	return c.SendEvent(TypeError, ErrorEventData{
		Code:            code,
		Message:         message,
		RequestType:     requestType,
		ClientMessageID: clientMessageID,
	})
}

func (c *Client) readPump() {
	defer c.hub.Unregister(c)

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Warn("websocket read failed", zap.Error(err))
			}
			return
		}

		if messageType != websocket.TextMessage {
			_ = c.SendError(ErrorBadRequest, "only text frames are supported", "", "")
			continue
		}

		c.log.Debug("websocket frame received",
			zap.Int("message_type", messageType),
			zap.Int("bytes", len(payload)),
		)
		c.hub.Touch(c)

		if c.messageHandler != nil {
			c.messageHandler.Handle(c, payload)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.log.Warn("websocket write failed", zap.Error(err))
				return
			}
		case <-ticker.C:
			c.hub.Touch(c)
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.log.Warn("websocket ping failed", zap.Error(err))
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		_ = c.conn.Close()
	})
}

func (c *Client) ConnectionSnapshot() ConnectionSnapshot {
	return ConnectionSnapshot{
		ConnectionID:   c.connectionID,
		UserUUID:       c.sessionUser.UUID,
		TokenID:        c.sessionUser.TokenID,
		TokenExpiresAt: c.sessionUser.TokenExpiresAt,
		Device:         c.identity.Device,
		DeviceID:       c.identity.DeviceID,
		UserAgent:      c.identity.UserAgent,
		RemoteAddr:     c.identity.RemoteAddr,
		ConnectedAt:    c.connectedAt,
		LastSeenAt:     time.Now().UTC(),
	}
}
