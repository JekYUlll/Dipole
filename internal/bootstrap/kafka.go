package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	aiModule "github.com/JekYUlll/Dipole/internal/modules/ai"
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
	DisconnectConnections(userUUID string, connectionIDs []string, reason string) int
	DisconnectAllConnections(userUUID string, reason string) int
}

type kafkaEventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}

func RegisterKafkaHandlers(hub kafkaWSEventSender) error {
	if platformKafka.Subscriber == nil {
		return nil
	}

	var events kafkaEventPublisher
	if platformKafka.Client != nil {
		events = platformKafka.Client
	}
	messageService := service.NewMessageService(
		repository.NewMessageRepository(),
		repository.NewUserRepository(),
		repository.NewContactRepository(),
		repository.NewGroupRepository(),
		service.NewFileService(repository.NewFileRepository(), repository.NewMessageRepository(), nil),
		events,
	)
	conversationService := service.NewConversationService(
		repository.NewConversationRepository(),
		repository.NewUserRepository(),
		repository.NewGroupRepository(),
		nil,
		events,
	)
	if aiService, err := newAIService(messageService); err != nil {
		return err
	} else if aiService != nil {
		platformKafka.Subscriber.Register("message.direct.created", handleAIDirectReply(aiService))
	}

	platformKafka.Subscriber.Register("message.direct.send_requested", persistMessageHandler(messageService, "direct"))
	platformKafka.Subscriber.Register("message.group.send_requested", persistMessageHandler(messageService, "group"))
	platformKafka.Subscriber.Register("message.direct.created", updateConversationHandler(conversationService, false))
	platformKafka.Subscriber.Register("message.group.created", updateConversationHandler(conversationService, true))
	if hub != nil {
		platformKafka.Subscriber.Register("group.created", deliverGroupEventHandler(hub, wsTransport.TypeGroupCreated, func(p service.GroupEventPayload) wsTransport.GroupCreatedEventData {
			return wsTransport.GroupCreatedEventData{
				GroupUUID:    p.GroupUUID,
				Name:         p.Name,
				Notice:       p.Notice,
				Avatar:       p.Avatar,
				MemberUUIDs:  p.MemberUUIDs,
				OperatorUUID: p.OperatorUUID,
				OccurredAt:   p.OccurredAt,
			}
		}))
		platformKafka.Subscriber.Register("message.direct.created", deliverDirectMessageHandler(hub))
		platformKafka.Subscriber.Register("message.group.created", deliverGroupMessageHandler(hub))
		platformKafka.Subscriber.Register("conversation.direct.read", deliverDirectReadHandler(hub))
		platformKafka.Subscriber.Register("group.updated", deliverGroupEventHandler(hub, wsTransport.TypeGroupUpdated, func(p service.GroupEventPayload) wsTransport.GroupUpdatedEventData {
			return wsTransport.GroupUpdatedEventData{
				GroupUUID:    p.GroupUUID,
				Name:         p.Name,
				Notice:       p.Notice,
				Avatar:       p.Avatar,
				OperatorUUID: p.OperatorUUID,
				UpdatedAt:    p.OccurredAt,
			}
		}))
		platformKafka.Subscriber.Register("group.members.added", deliverGroupEventHandler(hub, wsTransport.TypeGroupMembersAdded, func(p service.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
			return wsTransport.GroupMembersChangedEventData{
				GroupUUID:    p.GroupUUID,
				MemberUUIDs:  p.MemberUUIDs,
				OperatorUUID: p.OperatorUUID,
				OccurredAt:   p.OccurredAt,
			}
		}))
		platformKafka.Subscriber.Register("group.members.removed", deliverGroupEventHandler(hub, wsTransport.TypeGroupMembersRemoved, func(p service.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
			return wsTransport.GroupMembersChangedEventData{
				GroupUUID:    p.GroupUUID,
				MemberUUIDs:  p.MemberUUIDs,
				OperatorUUID: p.OperatorUUID,
				OccurredAt:   p.OccurredAt,
			}
		}))
		platformKafka.Subscriber.Register("group.dismissed", deliverGroupEventHandler(hub, wsTransport.TypeGroupDismissed, func(p service.GroupEventPayload) wsTransport.GroupDismissedEventData {
			return wsTransport.GroupDismissedEventData{
				GroupUUID:    p.GroupUUID,
				GroupName:    p.GroupName,
				OperatorUUID: p.OperatorUUID,
				OccurredAt:   p.OccurredAt,
			}
		}))
		platformKafka.Subscriber.Register("session.force_logout", deliverSessionKickHandler(hub))
	}
	for _, topic := range []string{"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed", "conversation.direct.read", "session.force_logout"} {
		platformKafka.Subscriber.Register(topic, logKafkaEventHandler(topic))
	}

	return nil
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

func persistMessageHandler(persister kafkaMessagePersister, label string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode "+label+" message requested payload failed", zap.Error(err))
			return err
		}

		if _, err := persister.PersistRequestedMessage(payload); err != nil {
			logger.Warn("persist "+label+" message from kafka failed", zap.Error(err))
			return err
		}

		logger.Info(label+" message persisted from kafka",
			zap.String("message_id", payload.MessageID),
			zap.Int64("offset", event.Offset),
		)
		return nil
	}
}

func updateConversationHandler(updater kafkaConversationUpdater, isGroup bool) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode message kafka event failed", zap.Error(err))
			return err
		}

		msg := servicePayloadToMessage(payload)
		var updateErr error
		if isGroup {
			updateErr = updater.UpdateGroupConversations(msg)
		} else {
			updateErr = updater.UpdateDirectConversations(msg)
		}
		if updateErr != nil {
			logger.Warn("update conversation from kafka failed", zap.Error(updateErr))
			return updateErr
		}

		logger.Info("conversation updated from kafka",
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

func newAIService(messageService *service.MessageService) (*aiModule.Service, error) {
	aiConfig := config.AIConfig()
	if !aiConfig.Enabled {
		return nil, nil
	}

	userRepo := repository.NewUserRepository()
	messageRepo := repository.NewMessageRepository()
	contextBuilder := aiModule.NewContextBuilder(
		messageRepo,
		userRepo,
		aiConfig.MaxContextMessages,
	)
	agent, err := aiModule.NewConfiguredAgent(
		context.Background(),
		aiModule.NewTools(contextBuilder, userRepo, messageRepo, messageService, aiConfig.AssistantUUID)...,
	)
	if err != nil {
		return nil, fmt.Errorf("init ai agent: %w", err)
	}

	return aiModule.NewService(
		contextBuilder,
		repository.NewAICallLogRepository(),
		messageService,
		agent,
	), nil
}

func handleAIDirectReply(aiService *aiModule.Service) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode ai trigger message payload failed", zap.Error(err))
			return err
		}

		if err := aiService.HandleDirectMessage(ctx, servicePayloadToMessage(payload)); err != nil {
			logger.Warn("handle ai direct reply failed",
				zap.String("message_id", payload.MessageID),
				zap.String("target_uuid", payload.TargetUUID),
				zap.Error(err),
			)
		}

		return nil
	}
}

// deliverGroupEventHandler is a generic factory for group event delivery handlers.
// It decodes the group event payload, builds the WS event data via buildData,
// and fans out to all recipients.
func deliverGroupEventHandler[T any](
	hub kafkaWSEventSender,
	eventType string,
	buildData func(service.GroupEventPayload) T,
) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode "+eventType+" payload failed", zap.Error(err))
			return err
		}

		data := buildData(payload)
		for _, recipientUUID := range payload.RecipientUUIDs {
			hub.SendEventToUser(recipientUUID, eventType, data)
		}

		return nil
	}
}

func deliverSessionKickHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeSessionKickPayload(event)
		if err != nil {
			logger.Warn("decode session kick payload failed", zap.Error(err))
			return err
		}
		if payload.All {
			hub.DisconnectAllConnections(payload.UserUUID, payload.Reason)
			return nil
		}

		hub.DisconnectConnections(payload.UserUUID, payload.ConnectionIDs, payload.Reason)
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

func decodeSessionKickPayload(event platformKafka.Event) (service.SessionKickEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.SessionKickEventPayload{}, err
	}

	var payload service.SessionKickEventPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return service.SessionKickEventPayload{}, fmt.Errorf("unmarshal session kick payload: %w", err)
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
		FileID:        payload.FileID,
		FileName:      payload.FileName,
		FileSize:      payload.FileSize,
		DownloadPath:  "/api/v1/files/" + payload.FileID + "/download",
		ContentType:   payload.FileContentType,
		FileExpiresAt: payload.FileExpiresAt,
	}
}
