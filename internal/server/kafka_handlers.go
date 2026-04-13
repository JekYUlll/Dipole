package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type kafkaConversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

type kafkaMessagePersister interface {
	PersistRequestedMessage(payload service.MessageEventPayload) (*model.Message, error)
}

type kafkaWSEventSender interface {
	SendEventToUser(userUUID, eventType string, data any) int
}

func RegisterKafkaHandlers(hub kafkaWSEventSender) {
	if platformKafka.Subscriber == nil {
		return
	}

	messageService := service.NewMessageService(
		repository.NewMessageRepository(),
		repository.NewUserRepository(),
		repository.NewContactRepository(),
		repository.NewGroupRepository(),
		service.NewFileService(repository.NewFileRepository(), nil),
		platformKafka.NewJSONPublisher(platformKafka.Client),
	)
	conversationService := service.NewConversationService(
		repository.NewConversationRepository(),
		repository.NewUserRepository(),
		repository.NewGroupRepository(),
	)

	platformKafka.Subscriber.Register("message.direct.send_requested", persistDirectMessageHandler(messageService))
	platformKafka.Subscriber.Register("message.group.send_requested", persistGroupMessageHandler(messageService))
	platformKafka.Subscriber.Register("message.direct.created", updateDirectConversationHandler(conversationService))
	platformKafka.Subscriber.Register("message.group.created", updateGroupConversationHandler(conversationService))
	if hub != nil {
		platformKafka.Subscriber.Register("message.direct.created", deliverDirectMessageHandler(hub))
		platformKafka.Subscriber.Register("message.group.created", deliverGroupMessageHandler(hub))
	}
	for _, topic := range []string{"group.created", "group.updated", "group.members.removed", "group.dismissed"} {
		platformKafka.Subscriber.Register(topic, logKafkaEventHandler(topic))
	}
}

func logKafkaEventHandler(topic string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		envelope, err := requireEnvelope(event)
		if err != nil {
			logger.Warn("decode kafka event envelope failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			return err
		}
		var payload map[string]any
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			logger.Warn("unmarshal kafka event payload failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			return err
		}

		logger.Info("kafka event consumed",
			zap.String("topic", event.Topic),
			zap.String("event_id", envelope.EventID),
			zap.String("event_type", envelope.EventType),
			zap.Int("partition", event.Partition),
			zap.Int64("offset", event.Offset),
			zap.Any("payload", payload),
		)
		return nil
	}
}

func persistDirectMessageHandler(persister kafkaMessagePersister) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode direct message requested payload failed", zap.Error(err))
			return err
		}

		if _, err := persister.PersistRequestedMessage(payload); err != nil {
			logger.Warn("persist direct message from kafka failed", zap.Error(err))
			return err
		}

		logger.Info("direct message persisted from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}

func persistGroupMessageHandler(persister kafkaMessagePersister) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode group message requested payload failed", zap.Error(err))
			return err
		}

		if _, err := persister.PersistRequestedMessage(payload); err != nil {
			logger.Warn("persist group message from kafka failed", zap.Error(err))
			return err
		}

		logger.Info("group message persisted from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}

func updateDirectConversationHandler(updater kafkaConversationUpdater) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode direct message kafka event failed", zap.Error(err))
			return err
		}

		if err := updater.UpdateDirectConversations(servicePayloadToMessage(payload)); err != nil {
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

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode group message kafka event failed", zap.Error(err))
			return err
		}

		if err := updater.UpdateGroupConversations(servicePayloadToMessage(payload)); err != nil {
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

func deliverDirectMessageHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode direct message for delivery failed", zap.Error(err))
			return err
		}

		hub.SendEventToUser(payload.TargetUUID, wsTransport.TypeChatMessage, wsTransport.ChatMessageData{
			MessageID:   payload.MessageID,
			FromUUID:    payload.SenderUUID,
			TargetUUID:  payload.TargetUUID,
			TargetType:  payload.TargetType,
			MessageType: payload.MessageType,
			Content:     payload.Content,
			File:        payloadToWSFile(payload),
			SentAt:      payload.SentAt,
		})

		return nil
	}
}

func deliverGroupMessageHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode group message for delivery failed", zap.Error(err))
			return err
		}

		eventData := wsTransport.ChatMessageData{
			MessageID:   payload.MessageID,
			FromUUID:    payload.SenderUUID,
			TargetUUID:  payload.TargetUUID,
			TargetType:  payload.TargetType,
			MessageType: payload.MessageType,
			Content:     payload.Content,
			File:        payloadToWSFile(payload),
			SentAt:      payload.SentAt,
		}
		for _, recipientUUID := range payload.RecipientUUIDs {
			if recipientUUID == payload.SenderUUID {
				continue
			}
			hub.SendEventToUser(recipientUUID, wsTransport.TypeChatMessage, eventData)
		}

		return nil
	}
}

func decodeMessageEventPayload(event platformKafka.Event) (service.MessageEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.MessageEventPayload{}, err
	}

	var payload service.MessageEventPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return service.MessageEventPayload{}, fmt.Errorf("unmarshal message event payload: %w", err)
	}

	return payload, nil
}

func requireEnvelope(event platformKafka.Event) (*platformKafka.Envelope, error) {
	if event.Envelope == nil {
		return nil, fmt.Errorf("kafka event envelope is missing")
	}

	return event.Envelope, nil
}

func servicePayloadToMessage(payload service.MessageEventPayload) *model.Message {
	return &model.Message{
		UUID:            payload.MessageID,
		ConversationKey: payload.ConversationKey,
		SenderUUID:      payload.SenderUUID,
		TargetUUID:      payload.TargetUUID,
		TargetType:      payload.TargetType,
		MessageType:     payload.MessageType,
		Content:         payload.Content,
		FileID:          payload.FileID,
		FileName:        payload.FileName,
		FileSize:        payload.FileSize,
		FileURL:         payload.FileURL,
		FileContentType: payload.FileContentType,
		SentAt:          payload.SentAt,
	}
}

func payloadToWSFile(payload service.MessageEventPayload) *wsTransport.FilePayload {
	if payload.MessageType != model.MessageTypeFile {
		return nil
	}

	return &wsTransport.FilePayload{
		FileID:      payload.FileID,
		FileName:    payload.FileName,
		FileSize:    payload.FileSize,
		FileURL:     payload.FileURL,
		ContentType: payload.FileContentType,
	}
}
