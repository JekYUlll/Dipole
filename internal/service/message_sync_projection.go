package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

// MessageSyncProjection maps the durable created-event contract to an Inbox locator.
func MessageSyncProjection(eventID, eventType string, payload MessageEventPayload) (*model.SyncProjection, bool, error) {
	expectedTarget := int8(-1)
	switch eventType {
	case "message.direct.created":
		expectedTarget = model.MessageTargetDirect
	case "message.group.created":
		expectedTarget = model.MessageTargetGroup
	default:
		return nil, false, fmt.Errorf("unsupported Sync projection event type %q", eventType)
	}
	if payload.TargetType != expectedTarget {
		return nil, false, fmt.Errorf("Sync event target type %d conflicts with %s", payload.TargetType, eventType)
	}
	messageUUID := strings.TrimSpace(payload.MessageID)
	conversationKey := strings.TrimSpace(payload.ConversationKey)
	if messageUUID == "" || conversationKey == "" || payload.MessageSeq == 0 {
		return nil, false, errors.New("Sync projection requires message_id, conversation_key, and message_seq")
	}
	recipients := append([]string(nil), payload.RecipientUUIDs...)
	if len(recipients) == 0 && expectedTarget == model.MessageTargetDirect {
		recipients = []string{payload.SenderUUID, payload.TargetUUID}
	}
	fanout := payload.SyncFanout == nil || *payload.SyncFanout
	if fanout && len(recipients) == 0 {
		return nil, false, errors.New("Sync group projection requires recipient snapshot")
	}
	return &model.SyncProjection{
		EventID: strings.TrimSpace(eventID), MessageUUID: messageUUID,
		ConversationKey: conversationKey, MessageSeq: payload.MessageSeq,
		RecipientUUIDs: recipients,
	}, fanout, nil
}
