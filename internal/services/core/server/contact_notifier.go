package server

import (
	"time"

	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type contactNotifier struct {
	hub *wsTransport.Hub
}

func newContactNotifier(hub *wsTransport.Hub) *contactNotifier {
	return &contactNotifier{hub: hub}
}

func (n *contactNotifier) NotifyFriendDeleted(userUUID, friendUUID string, occurredAt time.Time) {
	if n == nil || n.hub == nil {
		return
	}

	n.hub.SendEventToUser(userUUID, wsTransport.TypeContactFriendDeleted, wsTransport.ContactFriendDeletedEventData{
		UserUUID:   userUUID,
		FriendUUID: friendUUID,
		OccurredAt: occurredAt,
	})
}
