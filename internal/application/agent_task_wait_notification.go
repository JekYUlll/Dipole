package application

import (
	"encoding/json"
	"fmt"
	"strings"
)

func DecodeAgentTaskWaitingNotificationV1(eventType string, payload []byte) (AgentTaskWaitingNotificationV1, error) {
	if strings.TrimSpace(eventType) != AgentTaskWaitingEventTypeV1 {
		return AgentTaskWaitingNotificationV1{}, fmt.Errorf("unexpected Agent Task waiting event type %q", eventType)
	}
	var notification AgentTaskWaitingNotificationV1
	if err := json.Unmarshal(payload, &notification); err != nil {
		return AgentTaskWaitingNotificationV1{}, fmt.Errorf("decode Agent Task waiting payload: %w", err)
	}
	notification.TenantID = strings.TrimSpace(notification.TenantID)
	notification.PrincipalUUID = strings.TrimSpace(notification.PrincipalUUID)
	notification.TaskUUID = strings.TrimSpace(notification.TaskUUID)
	if notification.TenantID == "" || notification.PrincipalUUID == "" || notification.TaskUUID == "" || notification.Revision == 0 ||
		(notification.PendingKind != "input" && notification.PendingKind != "approval") {
		return AgentTaskWaitingNotificationV1{}, fmt.Errorf("Agent Task waiting payload is invalid")
	}
	return notification, nil
}
