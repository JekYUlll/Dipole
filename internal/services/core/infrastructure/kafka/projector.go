package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	"go.uber.org/zap"
)

type conversationProjector interface {
	InitGroupConversations(groupUUID string, memberUUIDs []string, createdAt time.Time) error
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

// RegisterConversationProjections registers only the Kafka projections owned
// by Core. Message persistence and Agent handlers belong to their services.
func RegisterConversationProjections(projector conversationProjector) error {
	if platformKafka.Subscriber == nil {
		return nil
	}
	if projector == nil {
		return fmt.Errorf("core conversation projector is required")
	}

	platformKafka.Subscriber.Register("group.created", initGroupConversations(projector))
	platformKafka.Subscriber.Register("message.direct.created", updateConversation(projector, false))
	platformKafka.Subscriber.Register("message.group.created", updateConversation(projector, true))
	for _, topic := range []string{"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed", "conversation.direct.read", "session.force_logout", "contact.friend.deleted"} {
		platformKafka.Subscriber.Register(topic, logEvent(topic))
	}
	return nil
}

func initGroupConversations(projector interface {
	InitGroupConversations(string, []string, time.Time) error
}) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx
		envelope, err := requireEnvelope(event)
		if err != nil {
			return err
		}
		payload, err := service.DecodeGroupEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode group event contract: %w", err)
		}
		if err := projector.InitGroupConversations(payload.GroupUUID, payload.MemberUUIDs, payload.OccurredAt); err != nil {
			return err
		}
		return nil
	}
}

func updateConversation(projector conversationProjector, group bool) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx
		envelope, err := requireEnvelope(event)
		if err != nil {
			return err
		}
		payload, err := messagedomain.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			return fmt.Errorf("decode message event contract: %w", err)
		}
		message := &model.Message{
			UUID: payload.MessageID, ConversationKey: payload.ConversationKey, Seq: payload.MessageSeq,
			SenderUUID: payload.SenderUUID, TargetType: payload.TargetType, TargetUUID: payload.TargetUUID,
			MessageType: payload.MessageType, Content: payload.Content, FileID: payload.FileID,
			FileName: payload.FileName, FileSize: payload.FileSize, FileURL: payload.FileURL,
			FileContentType: payload.FileContentType, SentAt: payload.SentAt,
		}
		if group {
			return projector.UpdateGroupConversations(message)
		}
		return projector.UpdateDirectConversations(message)
	}
}

func logEvent(topic string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx
		envelope, err := requireEnvelope(event)
		if err != nil {
			return err
		}
		var payload map[string]any
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return err
		}
		logger.Info("core kafka event consumed", zap.String("topic", topic), zap.String("event_id", envelope.EventID), zap.Any("payload", payload))
		return nil
	}
}

func requireEnvelope(event platformKafka.Event) (*platformKafka.Envelope, error) {
	if event.Envelope == nil {
		return nil, fmt.Errorf("kafka event envelope is missing")
	}
	return event.Envelope, nil
}
