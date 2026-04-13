package server

import wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"

type groupNotifier struct {
	hub *wsTransport.Hub
}

func newGroupNotifier(hub *wsTransport.Hub) *groupNotifier {
	return &groupNotifier{hub: hub}
}

func (n *groupNotifier) NotifyGroupDismissed(groupUUID, groupName string, memberUUIDs []string) {
	if n == nil || n.hub == nil {
		return
	}

	event := wsTransport.GroupDismissedEventData{
		GroupUUID: groupUUID,
		GroupName: groupName,
	}
	for _, memberUUID := range memberUUIDs {
		n.hub.SendEventToUser(memberUUID, wsTransport.TypeGroupDismissed, event)
	}
}
