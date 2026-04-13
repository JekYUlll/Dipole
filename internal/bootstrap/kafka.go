package bootstrap

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
		nil,
		platformKafka.NewJSONPublisher(platformKafka.Client),
	)

	platformKafka.Subscriber.Register("message.direct.send_requested", persistDirectMessageHandler(messageService))
	platformKafka.Subscriber.Register("message.group.send_requested", persistGroupMessageHandler(messageService))
	platformKafka.Subscriber.Register("message.direct.created", updateDirectConversationHandler(conversationService))
	platformKafka.Subscriber.Register("message.group.created", updateGroupConversationHandler(conversationService))
	if hub != nil {
		platformKafka.Subscriber.Register("message.direct.created", deliverDirectMessageHandler(hub))
		platformKafka.Subscriber.Register("message.group.created", deliverGroupMessageHandler(hub))
		platformKafka.Subscriber.Register("conversation.direct.read", deliverDirectReadHandler(hub))
		platformKafka.Subscriber.Register("group.updated", deliverGroupUpdatedHandler(hub))
		platformKafka.Subscriber.Register("group.members.added", deliverGroupMembersAddedHandler(hub))
		platformKafka.Subscriber.Register("group.members.removed", deliverGroupMembersRemovedHandler(hub))
		platformKafka.Subscriber.Register("group.dismissed", deliverGroupDismissedHandler(hub))
	}
	for _, topic := range []string{"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed", "conversation.direct.read"} {
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

func deliverDirectReadHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		envelope, err := requireEnvelope(event)
		if err != nil {
			logger.Warn("decode direct read envelope failed", zap.Error(err))
			return err
		}

		var payload service.ConversationReadReceipt
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			logger.Warn("decode direct read payload failed", zap.Error(err))
			return err
		}

		hub.SendEventToUser(payload.TargetUUID, wsTransport.TypeChatRead, wsTransport.ChatReadData{
			ReaderUUID:          payload.ReaderUUID,
			TargetUUID:          payload.TargetUUID,
			TargetType:          payload.TargetType,
			ConversationKey:     payload.ConversationKey,
			LastReadMessageUUID: payload.LastReadMessageUUID,
			ReadAt:              payload.ReadAt,
		})

		return nil
	}
}

func deliverGroupUpdatedHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode group updated payload failed", zap.Error(err))
			return err
		}

		eventData := wsTransport.GroupUpdatedEventData{
			GroupUUID:    payload.GroupUUID,
			Name:         payload.Name,
			Notice:       payload.Notice,
			Avatar:       payload.Avatar,
			OperatorUUID: payload.OperatorUUID,
			UpdatedAt:    payload.OccurredAt,
		}
		for _, recipientUUID := range payload.RecipientUUIDs {
			hub.SendEventToUser(recipientUUID, wsTransport.TypeGroupUpdated, eventData)
		}

		return nil
	}
}

func deliverGroupMembersAddedHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode group members added payload failed", zap.Error(err))
			return err
		}

		eventData := wsTransport.GroupMembersChangedEventData{
			GroupUUID:    payload.GroupUUID,
			MemberUUIDs:  payload.MemberUUIDs,
			OperatorUUID: payload.OperatorUUID,
			OccurredAt:   payload.OccurredAt,
		}
		for _, recipientUUID := range payload.RecipientUUIDs {
			hub.SendEventToUser(recipientUUID, wsTransport.TypeGroupMembersAdded, eventData)
		}

		return nil
	}
}

func deliverGroupMembersRemovedHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode group members removed payload failed", zap.Error(err))
			return err
		}

		eventData := wsTransport.GroupMembersChangedEventData{
			GroupUUID:    payload.GroupUUID,
			MemberUUIDs:  payload.MemberUUIDs,
			OperatorUUID: payload.OperatorUUID,
			OccurredAt:   payload.OccurredAt,
		}
		for _, recipientUUID := range payload.RecipientUUIDs {
			hub.SendEventToUser(recipientUUID, wsTransport.TypeGroupMembersRemoved, eventData)
		}

		return nil
	}
}

func deliverGroupDismissedHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode group dismissed payload failed", zap.Error(err))
			return err
		}

		eventData := wsTransport.GroupDismissedEventData{
			GroupUUID:    payload.GroupUUID,
			GroupName:    payload.GroupName,
			OperatorUUID: payload.OperatorUUID,
			OccurredAt:   payload.OccurredAt,
		}
		for _, recipientUUID := range payload.RecipientUUIDs {
			hub.SendEventToUser(recipientUUID, wsTransport.TypeGroupDismissed, eventData)
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

func decodeGroupEventPayload(event platformKafka.Event) (service.GroupEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.GroupEventPayload{}, err
	}

	var payload service.GroupEventPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return service.GroupEventPayload{}, fmt.Errorf("unmarshal group event payload: %w", err)
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
		TargetType:      payload.TargetType,
		TargetUUID:      payload.TargetUUID,
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
