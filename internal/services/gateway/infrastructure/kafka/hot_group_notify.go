package kafka

import (
	"sync"
	"time"

	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

const DefaultNotifyWindow = 200 * time.Millisecond

type EventSender interface {
	SendEventToUser(userUUID, eventType string, data any) int
}

type Notifier struct {
	hub    EventSender
	window time.Duration

	mu      sync.Mutex
	pending map[string]*pendingHotGroupNotify
}

type pendingHotGroupNotify struct {
	data       wsTransport.GroupMessageNotifyData
	recipients map[string]struct{}
	timer      *time.Timer
}

func NewNotifier(hub EventSender, window time.Duration) *Notifier {
	if hub == nil {
		return nil
	}
	if window <= 0 {
		window = DefaultNotifyWindow
	}

	return &Notifier{
		hub:     hub,
		window:  window,
		pending: make(map[string]*pendingHotGroupNotify),
	}
}

func (a *Notifier) Enqueue(groupUUID string, data wsTransport.GroupMessageNotifyData, recipients []string) {
	if a == nil || groupUUID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.pending[groupUUID]
	if !ok {
		entry = &pendingHotGroupNotify{
			data:       data,
			recipients: make(map[string]struct{}, len(recipients)),
		}
		entry.timer = time.AfterFunc(a.window, func() {
			a.flush(groupUUID)
		})
		a.pending[groupUUID] = entry
	}

	entry.data = data
	for _, recipientUUID := range recipients {
		if recipientUUID == "" {
			continue
		}
		entry.recipients[recipientUUID] = struct{}{}
	}
}

func (a *Notifier) flush(groupUUID string) {
	a.mu.Lock()
	entry := a.pending[groupUUID]
	if entry == nil {
		a.mu.Unlock()
		return
	}
	delete(a.pending, groupUUID)

	recipients := make([]string, 0, len(entry.recipients))
	for recipientUUID := range entry.recipients {
		recipients = append(recipients, recipientUUID)
	}
	data := entry.data
	a.mu.Unlock()

	for _, recipientUUID := range recipients {
		a.hub.SendEventToUser(recipientUUID, wsTransport.TypeGroupMessageNotify, data)
	}
}
