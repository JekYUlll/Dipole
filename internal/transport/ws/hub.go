package ws

import (
	"encoding/json"
	"sync"
	"time"
)

type Hub struct {
	mu       sync.RWMutex
	clients  map[string]map[*Client]struct{}
	presence PresenceTracker
}

type HubOption func(*Hub)

func NewHub(options ...HubOption) *Hub {
	hub := &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
	for _, option := range options {
		if option != nil {
			option(hub)
		}
	}

	return hub
}

func WithPresenceTracker(tracker PresenceTracker) HubOption {
	return func(h *Hub) {
		h.presence = tracker
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client.sessionUser.UUID]; !ok {
		h.clients[client.sessionUser.UUID] = make(map[*Client]struct{})
	}
	h.clients[client.sessionUser.UUID][client] = struct{}{}
	h.mu.Unlock()

	if h.presence != nil {
		h.presence.Register(client.ConnectionSnapshot())
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	if userClients, ok := h.clients[client.sessionUser.UUID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(h.clients, client.sessionUser.UUID)
		}
	}
	h.mu.Unlock()

	if h.presence != nil {
		h.presence.Unregister(client.sessionUser.UUID, client.connectionID)
	}
	client.Close()
}

func (h *Hub) UserConnectionCount(userUUID string) int {
	if h.presence != nil {
		if count := h.presence.UserConnectionCount(userUUID); count > 0 {
			return count
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients[userUUID])
}

func (h *Hub) OnlineUserCount() int {
	if h.presence != nil {
		if count := h.presence.OnlineUserCount(); count > 0 {
			return count
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

func (h *Hub) TotalConnectionCount() int {
	if h.presence != nil {
		if count := h.presence.TotalConnectionCount(); count > 0 {
			return count
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, userClients := range h.clients {
		total += len(userClients)
	}

	return total
}

func (h *Hub) Touch(client *Client) {
	if h == nil || client == nil || h.presence == nil {
		return
	}

	h.presence.Touch(client.ConnectionSnapshot())
}

func (h *Hub) DisconnectConnections(userUUID string, connectionIDs []string, reason string) int {
	if len(connectionIDs) == 0 {
		return 0
	}

	targets := make(map[string]struct{}, len(connectionIDs))
	for _, connectionID := range connectionIDs {
		if connectionID == "" {
			continue
		}
		targets[connectionID] = struct{}{}
	}
	if len(targets) == 0 {
		return 0
	}

	clients := h.snapshotClients(userUUID)
	disconnected := 0
	for _, client := range clients {
		if _, ok := targets[client.connectionID]; !ok {
			continue
		}
		disconnected += h.disconnectClient(client, reason)
	}

	return disconnected
}

func (h *Hub) DisconnectAllConnections(userUUID string, reason string) int {
	clients := h.snapshotClients(userUUID)
	disconnected := 0
	for _, client := range clients {
		disconnected += h.disconnectClient(client, reason)
	}

	return disconnected
}

func (h *Hub) SendEventToUser(userUUID string, eventType string, data any) int {
	payload, err := json.Marshal(OutboundEvent{
		Type: eventType,
		Data: data,
	})
	if err != nil {
		return 0
	}

	return h.sendToUser(userUUID, payload)
}

func (h *Hub) sendToUser(userUUID string, payload []byte) int {
	// TODO: 当在线连接数和群广播规模继续上来后，可在这一层引入 ants 协程池，
	// 统一收口 WS 批量投递任务，限制 goroutine 峰值并平滑广播压力。
	clients := h.snapshotClients(userUUID)
	delivered := 0
	for _, client := range clients {
		if err := client.Enqueue(payload); err == nil {
			delivered++
		}
	}

	return delivered
}

func (h *Hub) CloseAll(reason string) int {
	clients := h.snapshotAllClients()
	closed := 0
	for _, client := range clients {
		closed += h.disconnectClient(client, reason)
	}

	return closed
}

func (h *Hub) snapshotClients(userUUID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userClients := h.clients[userUUID]
	clients := make([]*Client, 0, len(userClients))
	for client := range userClients {
		clients = append(clients, client)
	}

	return clients
}

func (h *Hub) snapshotAllClients() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]*Client, 0)
	for _, userClients := range h.clients {
		for client := range userClients {
			clients = append(clients, client)
		}
	}

	return clients
}

func (h *Hub) disconnectClient(client *Client, reason string) int {
	if client == nil {
		return 0
	}

	_ = client.SendEvent(TypeSessionKicked, SessionKickedData{
		ConnectionID: client.connectionID,
		Reason:       reason,
		OccurredAt:   time.Now().UTC(),
	})
	h.Unregister(client)
	return 1
}
