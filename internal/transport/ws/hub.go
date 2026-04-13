package ws

import (
	"encoding/json"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.sessionUser.UUID]; !ok {
		h.clients[client.sessionUser.UUID] = make(map[*Client]struct{})
	}
	h.clients[client.sessionUser.UUID][client] = struct{}{}
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

	client.Close()
}

func (h *Hub) UserConnectionCount(userUUID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients[userUUID])
}

func (h *Hub) OnlineUserCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

func (h *Hub) TotalConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, userClients := range h.clients {
		total += len(userClients)
	}

	return total
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
	clients := h.snapshotClients(userUUID)
	delivered := 0
	for _, client := range clients {
		if err := client.Enqueue(payload); err == nil {
			delivered++
		}
	}

	return delivered
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
