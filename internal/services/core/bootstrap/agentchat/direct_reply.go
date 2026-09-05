// Package agentchat wires the legacy eino conversational assistant (package ai
// under internal/services/agent/legacy) onto the message.direct.created Kafka
// stream. It is the single composition home shared by the embedded monolith
// bootstrap and the microservices Core runtime so the "DM the assistant → it
// replies (with tools)" loop behaves identically in both deployments.
//
// It is inert unless AI is enabled in an embedded/shadow runtime mode
// (config.AI.RunsEmbeddedAgent); Route B will eventually fold this trigger into
// the governed agent-runtime and retire this package.
package agentchat

import (
	"context"
	"fmt"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	aiModule "github.com/JekYUlll/Dipole/internal/services/agent/legacy"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	"go.uber.org/zap"
)

// NewDirectReplyService composes the legacy conversational assistant Service.
// It returns (nil, nil) when AI is disabled or not in an embedded/shadow runtime
// mode, so callers can register the handler unconditionally and stay inert.
func NewDirectReplyService(
	aiConfig config.AI,
	logs applicationPort.AICallLogStore,
	commands applicationPort.AgentCommandV1,
	capability applicationPort.AgentCapabilityV1,
	policyStore applicationPort.AgentPolicyStoreV1,
) (*aiModule.Service, error) {
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
		executionPolicy, err = agentapplication.NewStaticAgentExecutionPolicyV1(permissions, scopes)
	case config.AIPolicyPersistent:
		if policyStore == nil {
			return nil, fmt.Errorf("Agent Policy Store v1 is required in persistent policy mode")
		}
		if err = agentapplication.EnsureEmbeddedAgentDefinitionV1(context.Background(), policyStore, "dipole", aiConfig.AssistantUUID, permissions, scopes); err == nil {
			executionPolicy, err = agentapplication.NewPersistentAgentExecutionPolicyV1(policyStore)
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

// DirectReplyHandler adapts the assistant Service onto the Kafka handler
// contract. It swallows reply errors (already logged) so a failed AI turn never
// blocks the message.direct.created consumer for other subscribers.
func DirectReplyHandler(aiService *aiModule.Service) platformKafka.Handler {
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

func decodeMessageEventPayload(event platformKafka.Event) (messagedomain.MessageEventPayload, error) {
	if event.Envelope == nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("kafka event envelope is missing")
	}
	payload, err := messagedomain.DecodeMessageEventPayload(event.Envelope.EventType, event.Envelope.Payload)
	if err != nil {
		return messagedomain.MessageEventPayload{}, fmt.Errorf("decode message event contract: %w", err)
	}
	return payload, nil
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
