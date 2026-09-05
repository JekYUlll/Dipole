package embedded

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	agentchat "github.com/JekYUlll/Dipole/internal/services/core/bootstrap/agentchat"
	coregroup "github.com/JekYUlll/Dipole/internal/services/core/domain/group"
	gatewaykafka "github.com/JekYUlll/Dipole/internal/services/gateway/infrastructure/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	messagekafka "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/kafka"
	"go.uber.org/zap"
)

type kafkaConversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

type WSEventSender interface {
	SendEventToUser(userUUID, eventType string, data any) int
	DisconnectConnections(userUUID string, connectionIDs []string, reason string) int
	DisconnectAllConnections(userUUID string, reason string) int
}

// CoreOwnsMessagePersistence reports whether the embedded composition owns
// message persistence instead of delegating it to the remote message service.
func CoreOwnsMessagePersistence(gatewayMode, messageTransport string) bool {
	return gatewayMode == "embedded" && messageTransport != "grpc"
}

type kafkaWSContextEventSender interface {
	SendEventToUserContext(ctx context.Context, userUUID, eventType string, data any) int
}

func sendEventToUser(ctx context.Context, hub WSEventSender, userUUID, eventType string, data any) int {
	if contextual, ok := hub.(kafkaWSContextEventSender); ok {
		return contextual.SendEventToUserContext(ctx, userUUID, eventType, data)
	}
	return hub.SendEventToUser(userUUID, eventType, data)
}

type kafkaGroupConversationIniter interface {
	InitGroupConversations(groupUUID string, memberUUIDs []string, createdAt time.Time) error
}

func RegisterKafkaHandlers(hub WSEventSender, repos *Repositories, messaging *MessagingServices, includeMessagePersistence bool) error {
	if platformKafka.Subscriber == nil {
		return nil
	}
	if repos == nil {
		return fmt.Errorf("kafka handler repositories are required")
	}
	if repos.AgentProcess == nil {
		return fmt.Errorf("kafka handler Agent repositories are required")
	}

	var events applicationPort.EventPublisher
	if platformKafka.Client != nil {
		events = platformKafka.Client
	}
	hotGroupDetector := platformHotGroup.NewDetectorWithClient(config.HotGroupConfig(), cache.RDB)
	if messaging == nil {
		messaging = NewMessagingServices(repos, MessagingDependencies{
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
		agentCommands, err := agentapplication.NewLocalAgentCommandV1(messaging.Messages)
		if err != nil {
			return fmt.Errorf("compose Agent Command v1: %w", err)
		}
		agentCapability, err := agentapplication.NewLocalAgentCapabilityV1(
			messaging.Core,
			messaging.Messages,
			messaging.Conversations,
			agentCommands,
		)
		if err != nil {
			return fmt.Errorf("compose Agent Capability v1: %w", err)
		}
		if aiService, err := agentchat.NewDirectReplyService(aiConfig, repos.AgentProcess.AICallLogs, agentCommands, agentCapability, repos.AgentProcess.Policy); err != nil {
			return err
		} else if aiService != nil {
			aiService.SetGroupMessenger(messaging.Messages)
			platformKafka.Subscriber.Register("message.direct.created", agentchat.DirectReplyHandler(aiService))
			platformKafka.Subscriber.Register("message.group.created", agentchat.GroupReplyHandler(aiService))
		}
	}
	if includeMessagePersistence {
		messagekafka.RegisterPersistenceHandlers(platformKafka.Subscriber, messaging.Messages)
	}
	platformKafka.Subscriber.Register("message.direct.created", updateConversationHandler(messaging.Conversations, false))
	platformKafka.Subscriber.Register("message.group.created", updateConversationHandler(messaging.Conversations, true))
	if hub != nil {
		authority, err := realtimeDelivery.ParseAuthority(config.RealtimeConfig().Delivery)
		if err != nil {
			return err
		}
		if err := gatewaykafka.RegisterHandlers(hub, authority, nil); err != nil {
			return err
		}
	}
	for _, topic := range []string{"group.created", "group.updated", "group.members.added", "group.members.removed", "group.dismissed", "conversation.direct.read", "session.force_logout"} {
		platformKafka.Subscriber.Register(topic, logKafkaEventHandler(topic))
	}
	platformKafka.Subscriber.Register("contact.friend.deleted", logKafkaEventHandler("contact.friend.deleted"))

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

func decodeMessageEventPayload(event platformKafka.Event) (messagedomain.MessageEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return messagedomain.MessageEventPayload{}, err
	}

	payload, err := messagedomain.DecodeMessageEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("decode message event contract: %w", err)
	}

	return payload, nil
}

func decodeGroupEventPayload(event platformKafka.Event) (coregroup.GroupEventPayload, error) {
	envelope, err := requireEnvelope(event)
	if err != nil {
		return coregroup.GroupEventPayload{}, err
	}

	payload, err := coregroup.DecodeEventPayload(envelope.EventType, envelope.Payload)
	if err != nil {
		return coregroup.GroupEventPayload{}, fmt.Errorf("unmarshal group event payload: %w", err)
	}

	return payload, nil
}

func requireEnvelope(event platformKafka.Event) (*platformKafka.Envelope, error) {
	if event.Envelope == nil {
		return nil, fmt.Errorf("kafka event envelope is missing")
	}

	return event.Envelope, nil
}

func servicePayloadToMessage(payload messagedomain.MessageEventPayload) *model.Message {
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
