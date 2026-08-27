package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	appComposition "github.com/JekYUlll/Dipole/internal/app"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	aiModule "github.com/JekYUlll/Dipole/internal/modules/ai"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
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

type kafkaMessagePersisterContext interface {
	PersistRequestedMessageContext(ctx context.Context, payload service.MessageEventPayload) (*model.Message, error)
}

type kafkaWSEventSender interface {
	SendEventToUser(userUUID, eventType string, data any) int
	DisconnectConnections(userUUID string, connectionIDs []string, reason string) int
	DisconnectAllConnections(userUUID string, reason string) int
}

type kafkaWSContextEventSender interface {
	SendEventToUserContext(ctx context.Context, userUUID, eventType string, data any) int
}

func sendEventToUser(ctx context.Context, hub kafkaWSEventSender, userUUID, eventType string, data any) int {
	if contextual, ok := hub.(kafkaWSContextEventSender); ok {
		return contextual.SendEventToUserContext(ctx, userUUID, eventType, data)
	}
	return hub.SendEventToUser(userUUID, eventType, data)
}

type kafkaGroupConversationIniter interface {
	InitGroupConversations(groupUUID string, memberUUIDs []string, createdAt time.Time) error
}

type groupHeatReader interface {
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

func RegisterKafkaHandlersWithRepositories(hub kafkaWSEventSender, repos *appComposition.Repositories) error {
	return registerCoreKafkaHandlers(hub, repos, nil, true)
}

func RegisterCoreKafkaHandlersWithRepositories(hub kafkaWSEventSender, repos *appComposition.Repositories) error {
	return registerCoreKafkaHandlers(hub, repos, nil, false)
}

func registerCoreKafkaHandlers(hub kafkaWSEventSender, repos *appComposition.Repositories, messaging *appComposition.MessagingServices, includeMessagePersistence bool) error {
	if platformKafka.Subscriber == nil {
		return nil
	}
	if repos == nil {
		return fmt.Errorf("kafka handler repositories are required")
	}

	var events applicationPort.EventPublisher
	if platformKafka.Client != nil {
		events = platformKafka.Client
	}
	hotGroupDetector := platformHotGroup.NewRedisDetector()
	if messaging == nil {
		messaging = appComposition.NewMessagingServices(repos, appComposition.MessagingDependencies{
			Events:    events,
			HotGroups: hotGroupDetector,
		})
	}
	platformKafka.Subscriber.Register("group.created", initGroupConversationHandler(messaging.Conversations))
	aiConfig := config.AIConfig()
	runsEmbeddedAgent, err := aiConfig.RunsEmbeddedAgent()
	if err != nil {
		return fmt.Errorf("resolve AI runtime mode: %w", err)
	}
	if runsEmbeddedAgent {
		agentCommands, err := appComposition.NewLocalAgentCommandV1(messaging.Messages)
		if err != nil {
			return fmt.Errorf("compose Agent Command v1: %w", err)
		}
		agentCapability, err := appComposition.NewLocalAgentCapabilityV1(
			messaging.Core,
			messaging.Messages,
			messaging.Conversations,
			agentCommands,
		)
		if err != nil {
			return fmt.Errorf("compose Agent Capability v1: %w", err)
		}
		if aiService, err := newAIService(aiConfig, repos.AICallLogs, agentCommands, agentCapability, repos.AgentPolicy); err != nil {
			return err
		} else if aiService != nil {
			platformKafka.Subscriber.Register("message.direct.created", handleAIDirectReply(aiService))
		}
	}
	if includeMessagePersistence {
		RegisterMessageKafkaHandlers(messaging.Messages)
	}
	platformKafka.Subscriber.Register("message.direct.created", updateConversationHandler(messaging.Conversations, false))
	platformKafka.Subscriber.Register("message.group.created", updateConversationHandler(messaging.Conversations, true))
	if hub != nil {
		registerGatewayKafkaHandlers(hub)
	}
	for _, topic := range []string{"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed", "conversation.direct.read", "session.force_logout"} {
		platformKafka.Subscriber.Register(topic, logKafkaEventHandler(topic))
	}
	platformKafka.Subscriber.Register("contact.friend.deleted", logKafkaEventHandler("contact.friend.deleted"))

	return nil
}

func RegisterGatewayKafkaHandlers(hub kafkaWSEventSender) error {
	if platformKafka.Subscriber == nil {
		return nil
	}
	if hub == nil {
		return fmt.Errorf("gateway kafka event sender is required")
	}
	registerGatewayKafkaHandlers(hub)
	return nil
}

func registerGatewayKafkaHandlers(hub kafkaWSEventSender) {
	hotGroups := platformHotGroup.NewRedisDetector()
	notifier := newHotGroupNotifyAggregator(hub, hotGroupNotifyWindow)
	platformKafka.Subscriber.Register("group.created", deliverGroupEventHandler(hub, wsTransport.TypeGroupCreated, func(p service.GroupEventPayload) wsTransport.GroupCreatedEventData {
		return wsTransport.GroupCreatedEventData{
			GroupUUID: p.GroupUUID, Name: p.Name, Notice: p.Notice, Avatar: p.Avatar,
			MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt,
		}
	}))
	timelineNotifyMode := config.MessageConfig().TimelineNotifyMode
	platformKafka.Subscriber.Register("message.direct.created", deliverDirectMessageHandler(hub, timelineNotifyMode))
	platformKafka.Subscriber.Register("message.group.created", deliverGroupMessageHandler(hub, hotGroups, notifier, timelineNotifyMode))
	platformKafka.Subscriber.Register("conversation.direct.read", deliverDirectReadHandler(hub))
	platformKafka.Subscriber.Register("group.updated", deliverGroupEventHandler(hub, wsTransport.TypeGroupUpdated, func(p service.GroupEventPayload) wsTransport.GroupUpdatedEventData {
		return wsTransport.GroupUpdatedEventData{
			GroupUUID: p.GroupUUID, Name: p.Name, Notice: p.Notice, Avatar: p.Avatar,
			OperatorUUID: p.OperatorUUID, UpdatedAt: p.OccurredAt,
		}
	}))
	platformKafka.Subscriber.Register("group.members.added", deliverGroupEventHandler(hub, wsTransport.TypeGroupMembersAdded, func(p service.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
		return wsTransport.GroupMembersChangedEventData{
			GroupUUID: p.GroupUUID, MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt,
		}
	}))
	platformKafka.Subscriber.Register("group.members.removed", deliverGroupEventHandler(hub, wsTransport.TypeGroupMembersRemoved, func(p service.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
		return wsTransport.GroupMembersChangedEventData{
			GroupUUID: p.GroupUUID, MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt,
		}
	}))
	platformKafka.Subscriber.Register("group.dismissed", deliverGroupEventHandler(hub, wsTransport.TypeGroupDismissed, func(p service.GroupEventPayload) wsTransport.GroupDismissedEventData {
		return wsTransport.GroupDismissedEventData{
			GroupUUID: p.GroupUUID, GroupName: p.GroupName, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt,
		}
	}))
	platformKafka.Subscriber.Register("session.force_logout", deliverSessionKickHandler(hub))
	platformKafka.Subscriber.Register("contact.friend.deleted", deliverContactFriendDeletedHandler(hub))
}

func RegisterMessageKafkaHandlers(persister kafkaMessagePersister) {
	if platformKafka.Subscriber == nil || persister == nil {
		return
	}
	platformKafka.Subscriber.Register("message.direct.send_requested", persistMessageHandler(persister, "direct"))
	platformKafka.Subscriber.Register("message.group.send_requested", persistMessageHandler(persister, "group"))
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
		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode "+label+" message requested payload failed", zap.Error(err))
			return err
		}

		if contextual, ok := persister.(kafkaMessagePersisterContext); ok {
			_, err = contextual.PersistRequestedMessageContext(ctx, payload)
		} else {
			_, err = persister.PersistRequestedMessage(payload)
		}
		if err != nil {
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

func deliverDirectMessageHandler(hub kafkaWSEventSender, timelineNotifyMode string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode direct message for delivery failed", zap.Error(err))
			return err
		}

		sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeChatMessage, wsTransport.ChatMessageData{
			MessageID:   payload.MessageID,
			MessageSeq:  payload.MessageSeq,
			FromUUID:    payload.SenderUUID,
			TargetUUID:  payload.TargetUUID,
			TargetType:  payload.TargetType,
			MessageType: payload.MessageType,
			Content:     payload.Content,
			File:        payloadToWSFile(payload),
			SentAt:      payload.SentAt,
		})
		if notify, ok := timelineNotifyData(event.Envelope, payload, timelineNotifyMode); ok {
			sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeSyncItemNotifyV1, notify)
		}

		return nil
	}
}

func deliverGroupMessageHandler(hub kafkaWSEventSender, hotGroups groupHeatReader, notifier *hotGroupNotifyAggregator, timelineNotifyMode string) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode group message for delivery failed", zap.Error(err))
			return err
		}

		// For hot groups (high message frequency), send a lightweight notify event instead of
		// the full message payload. The client then batch-fetches missed messages via REST,
		// avoiding WS fan-out storms when many members are online simultaneously.
		hot := false
		recentMessageCount := 0
		if hotGroups != nil {
			status, statusErr := hotGroups.Status(payload.TargetUUID, len(payload.RecipientUUIDs))
			if statusErr != nil {
				logger.Warn("query hot group status failed",
					zap.String("group_uuid", payload.TargetUUID),
					zap.Error(statusErr),
				)
			} else {
				hot = status.IsHot
				recentMessageCount = status.RecentMessageCount
			}
		}

		eventData := wsTransport.ChatMessageData{
			MessageID:   payload.MessageID,
			MessageSeq:  payload.MessageSeq,
			FromUUID:    payload.SenderUUID,
			TargetUUID:  payload.TargetUUID,
			TargetType:  payload.TargetType,
			MessageType: payload.MessageType,
			Content:     payload.Content,
			File:        payloadToWSFile(payload),
			SentAt:      payload.SentAt,
		}
		var wg sync.WaitGroup
		// 这里仍然保留 per-recipient fan-out，是为了复用现有的用户级连接路由能力。
		// 热群模式下只把正文换成 notify，先把 WS 写放大降下来；后续如果继续压测，
		// 这一层还可以演进成按 node_id 聚合后再批量转发。
		for _, recipientUUID := range payload.RecipientUUIDs {
			if hot {
				continue
			}
			if recipientUUID == payload.SenderUUID {
				continue
			}
			wg.Add(1)
			go func(uuid string) {
				defer wg.Done()
				sendEventToUser(ctx, hub, uuid, wsTransport.TypeChatMessage, eventData)
				if notify, ok := timelineNotifyData(event.Envelope, payload, timelineNotifyMode); ok {
					sendEventToUser(ctx, hub, uuid, wsTransport.TypeSyncItemNotifyV1, notify)
				}
			}(recipientUUID)
		}
		wg.Wait()
		if hot {
			// 热群下不再逐条立即 fan-out notify，而是把短时间窗口内的多条消息
			// 合并成一次“最新游标通知”，让客户端做一次批量补拉即可。
			notifier.Enqueue(payload.TargetUUID, wsTransport.GroupMessageNotifyData{
				GroupUUID:          payload.TargetUUID,
				LatestMessageID:    payload.MessageID,
				LatestMessageSeq:   payload.MessageSeq,
				MessageType:        payload.MessageType,
				Preview:            messagePreview(payload),
				RecentMessageCount: recentMessageCount,
				SentAt:             payload.SentAt,
				SenderUUID:         payload.SenderUUID,
			}, payload.RecipientUUIDs)
		}

		return nil
	}
}

func timelineNotifyData(envelope *platformKafka.Envelope, payload service.MessageEventPayload, mode string) (wsTransport.SyncItemNotifyData, bool) {
	if mode != wsTransport.TimelineNotifyShadow || payload.MessageSeq == 0 || strings.TrimSpace(payload.MessageID) == "" || strings.TrimSpace(payload.ConversationKey) == "" {
		return wsTransport.SyncItemNotifyData{}, false
	}
	eventID := payload.MessageID
	if envelope != nil && strings.TrimSpace(envelope.EventID) != "" {
		eventID = strings.TrimSpace(envelope.EventID)
	}
	return wsTransport.SyncItemNotifyData{
		SchemaVersion: "v1", EventID: eventID, MessageUUID: payload.MessageID,
		ConversationKey: payload.ConversationKey, MessageSeq: payload.MessageSeq,
		TargetType: payload.TargetType, TargetUUID: payload.TargetUUID,
	}, true
}

func messagePreview(payload service.MessageEventPayload) string {
	if payload.MessageType == model.MessageTypeFile {
		if payload.FileName != "" {
			return "[文件] " + payload.FileName
		}
		return "[文件]"
	}
	return payload.Content
}

func deliverDirectReadHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		envelope, err := requireEnvelope(event)
		if err != nil {
			logger.Warn("decode direct read envelope failed", zap.Error(err))
			return err
		}

		payload, err := service.DecodeConversationReadReceipt(envelope.EventType, envelope.Payload)
		if err != nil {
			logger.Warn("decode direct read payload failed", zap.Error(err))
			return err
		}

		sendEventToUser(ctx, hub, payload.TargetUUID, wsTransport.TypeChatRead, wsTransport.ChatReadData{
			ReaderUUID:          payload.ReaderUUID,
			TargetUUID:          payload.TargetUUID,
			TargetType:          payload.TargetType,
			ConversationKey:     payload.ConversationKey,
			LastReadMessageUUID: payload.LastReadMessageUUID,
			LastReadSeq:         payload.LastReadSeq,
			ReadAt:              payload.ReadAt,
		})

		return nil
	}
}

func newAIService(aiConfig config.AI, logs applicationPort.AICallLogStore, commands applicationPort.AgentCommandV1, capability applicationPort.AgentCapabilityV1, policyStore applicationPort.AgentPolicyStoreV1) (*aiModule.Service, error) {
	runsEmbeddedAgent, err := aiConfig.RunsEmbeddedAgent()
	if err != nil {
		return nil, fmt.Errorf("resolve AI runtime mode: %w", err)
	}
	if !runsEmbeddedAgent {
		return nil, nil
	}
	if capability == nil {
		return nil, fmt.Errorf("Agent Capability v1 is required when AI is enabled")
	}
	if logs == nil {
		return nil, fmt.Errorf("AI call log store is required when AI is enabled")
	}
	if commands == nil {
		return nil, fmt.Errorf("Agent Command v1 is required when AI is enabled")
	}
	policyMode, err := aiConfig.ResolvedPolicyMode()
	if err != nil {
		return nil, fmt.Errorf("resolve AI policy mode: %w", err)
	}
	permissions, scopes := applicationPort.EmbeddedAgentPolicyGrantV1()
	var executionPolicy applicationPort.AgentExecutionPolicyV1
	switch policyMode {
	case config.AIPolicyStatic:
		executionPolicy, err = appComposition.NewStaticAgentExecutionPolicyV1(permissions, scopes)
	case config.AIPolicyPersistent:
		if policyStore == nil {
			return nil, fmt.Errorf("Agent Policy Store v1 is required in persistent policy mode")
		}
		if err = appComposition.EnsureEmbeddedAgentDefinitionV1(context.Background(), policyStore, "dipole", aiConfig.AssistantUUID, permissions, scopes); err == nil {
			executionPolicy, err = appComposition.NewPersistentAgentExecutionPolicyV1(policyStore)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("compose Agent execution policy: %w", err)
	}

	contextBuilder := aiModule.NewContextBuilder(capability, aiConfig.MaxContextMessages)
	agent, err := aiModule.NewConfiguredAgent(
		context.Background(),
		aiModule.NewTools(capability, aiConfig.AssistantUUID)...,
	)
	if err != nil {
		return nil, fmt.Errorf("init ai agent: %w", err)
	}

	return aiModule.NewService(
		contextBuilder,
		logs,
		commands,
		executionPolicy,
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
			sendEventToUser(ctx, hub, recipientUUID, eventType, data)
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

func deliverContactFriendDeletedHandler(hub kafkaWSEventSender) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx

		envelope, err := requireEnvelope(event)
		if err != nil {
			logger.Warn("decode contact friend deleted envelope failed", zap.Error(err))
			return err
		}

		payload, err := service.DecodeContactFriendDeletedPayload(envelope.EventType, envelope.Payload)
		if err != nil {
			logger.Warn("decode contact friend deleted payload failed", zap.Error(err))
			return err
		}

		sendEventToUser(ctx, hub, payload.UserUUID, wsTransport.TypeContactFriendDeleted, wsTransport.ContactFriendDeletedEventData{
			UserUUID:   payload.UserUUID,
			FriendUUID: payload.FriendUUID,
			OccurredAt: payload.OccurredAt,
		})
		return nil
	}
}

func decodeMessageEventPayload(event platformKafka.Event) (service.MessageEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.MessageEventPayload{}, err
	}

	payload, err := service.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
		return service.MessageEventPayload{}, fmt.Errorf("decode message event contract: %w", err)
	}

	return payload, nil
}

func decodeGroupEventPayload(event platformKafka.Event) (service.GroupEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.GroupEventPayload{}, err
	}

	payload, err := service.DecodeGroupEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
		return service.GroupEventPayload{}, fmt.Errorf("unmarshal group event payload: %w", err)
	}

	return payload, nil
}

func decodeSessionKickPayload(event platformKafka.Event) (service.SessionKickEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return service.SessionKickEventPayload{}, err
	}

	payload, err := service.DecodeSessionKickEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
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
		Seq:             payload.MessageSeq,
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

// initGroupConversationHandler handles the group.created Kafka event.
// It seeds an empty conversation row for every group member so the group
// appears in their conversation list immediately, before any message is sent.
func initGroupConversationHandler(initer kafkaGroupConversationIniter) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		_ = ctx
		payload, err := decodeGroupEventPayload(event)
		if err != nil {
			logger.Warn("decode group.created payload for conversation init failed", zap.Error(err))
			return err
		}
		if err := initer.InitGroupConversations(payload.GroupUUID, payload.MemberUUIDs, payload.OccurredAt); err != nil {
			logger.Warn("init group conversations failed",
				zap.String("group_uuid", payload.GroupUUID),
				zap.Error(err),
			)
			return err
		}
		logger.Info("group conversations initialized",
			zap.String("group_uuid", payload.GroupUUID),
			zap.Int("member_count", len(payload.MemberUUIDs)),
		)
		return nil
	}
}
