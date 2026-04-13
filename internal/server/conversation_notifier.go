package server

import (
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type conversationNotifier struct {
	hub *wsTransport.Hub
}

func newConversationNotifier(hub *wsTransport.Hub) *conversationNotifier {
	return &conversationNotifier{hub: hub}
}

func (n *conversationNotifier) NotifyDirectRead(receipt service.ConversationReadReceipt) {
	if n == nil || n.hub == nil {
		return
	}

	n.hub.SendEventToUser(receipt.TargetUUID, wsTransport.TypeChatRead, wsTransport.ChatReadData{
		ReaderUUID:          receipt.ReaderUUID,
		TargetUUID:          receipt.TargetUUID,
		TargetType:          receipt.TargetType,
		ConversationKey:     receipt.ConversationKey,
		LastReadMessageUUID: receipt.LastReadMessageUUID,
		ReadAt:              receipt.ReadAt,
	})
}
