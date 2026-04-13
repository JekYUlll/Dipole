package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/service"
	"go.uber.org/zap"
)

func RegisterKafkaHandlers() {
	if platformKafka.Subscriber == nil {
		return
	}

	conversationService := service.NewConversationService(
		repository.NewConversationRepository(),
		repository.NewUserRepository(),
		repository.NewGroupRepository(),
	)

	platformKafka.Subscriber.Register("message.direct.created", updateDirectConversationHandler(conversationService))
	platformKafka.Subscriber.Register("message.group.created", updateGroupConversationHandler(conversationService))
	for _, topic := range []string{"group.created", "group.updated", "group.members.removed", "group.dismissed"} {
		platformKafka.Subscriber.Register(topic, logKafkaEventHandler(topic))
	}
}

func logKafkaEventHandler(topic string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		var payload map[string]any
		if err := json.Unmarshal(event.Value, &payload); err != nil {
			logger.Warn("unmarshal kafka event failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			return err
		}

		logger.Info("kafka event consumed",
			zap.String("topic", topic),
			zap.Int("partition", event.Partition),
			zap.Int64("offset", event.Offset),
			zap.Any("payload", payload),
		)
		return nil
	}
}

type messageCreatedPayload struct {
	MessageID       string    `json:"message_id"`
	ConversationKey string    `json:"conversation_key"`
	SenderUUID      string    `json:"sender_uuid"`
	TargetUUID      string    `json:"target_uuid"`
	TargetType      int8      `json:"target_type"`
	MessageType     int8      `json:"message_type"`
	Content         string    `json:"content"`
	SentAt          time.Time `json:"sent_at"`
}

type kafkaConversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

func updateDirectConversationHandler(updater kafkaConversationUpdater) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		var payload messageCreatedPayload
		if err := json.Unmarshal(event.Value, &payload); err != nil {
			logger.Warn("unmarshal direct message kafka event failed", zap.Error(err))
			return err
		}

		message := &model.Message{
			UUID:            payload.MessageID,
			ConversationKey: payload.ConversationKey,
			SenderUUID:      payload.SenderUUID,
			TargetUUID:      payload.TargetUUID,
			TargetType:      payload.TargetType,
			MessageType:     payload.MessageType,
			Content:         payload.Content,
			SentAt:          payload.SentAt,
		}
		if err := updater.UpdateDirectConversations(message); err != nil {
			logger.Warn("update direct conversation from kafka failed", zap.Error(err))
			return err
		}

		logger.Info("direct conversation updated from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}

func updateGroupConversationHandler(updater kafkaConversationUpdater) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		var payload messageCreatedPayload
		if err := json.Unmarshal(event.Value, &payload); err != nil {
			logger.Warn("unmarshal group message kafka event failed", zap.Error(err))
			return err
		}

		message := &model.Message{
			UUID:            payload.MessageID,
			ConversationKey: payload.ConversationKey,
			SenderUUID:      payload.SenderUUID,
			TargetUUID:      payload.TargetUUID,
			TargetType:      payload.TargetType,
			MessageType:     payload.MessageType,
			Content:         payload.Content,
			SentAt:          payload.SentAt,
		}
		if err := updater.UpdateGroupConversations(message); err != nil {
			logger.Warn("update group conversation from kafka failed", zap.Error(err))
			return err
		}

		logger.Info("group conversation updated from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}
