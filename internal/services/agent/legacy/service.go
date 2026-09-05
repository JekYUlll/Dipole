package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
)

var (
	ErrAIAssistantNotFound = errors.New("ai assistant not found")
	ErrAIUserNotFound      = errors.New("ai conversation user not found")
)

type callLogRepository interface {
	Begin(log *model.AICallLog) (bool, error)
	MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error
	MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error
}

type conversationContextBuilder interface {
	BuildDirectContext(ctx context.Context, userUUID, assistantUUID string) (*ConversationContext, error)
	BuildGroupContext(ctx context.Context, userUUID, assistantUUID, groupUUID string) (*ConversationContext, error)
}

// GroupReplySender is the trusted write used for Route A · A2 group replies.
// The existing AgentCommand v1 assistant_reply path is 1v1-only. Sending as
// the assistant (not an empty system actor) keeps realtime delivery valid
// and requires the assistant to be a group member.
type GroupReplySender interface {
	SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error)
}

type Service struct {
	config         config.AI
	contextBuilder conversationContextBuilder
	logs           callLogRepository
	commands       application.AgentCommandV1
	policy         application.AgentExecutionPolicyV1
	agent          Agent
	groupMessages  GroupReplySender
}

func NewService(builder conversationContextBuilder, logs callLogRepository, commands application.AgentCommandV1, policy application.AgentExecutionPolicyV1, agent Agent) *Service {
	return &Service{
		config:         config.AIConfig(),
		contextBuilder: builder,
		logs:           logs,
		commands:       commands,
		policy:         policy,
		agent:          agent,
	}
}

func (s *Service) SetGroupMessenger(messenger GroupReplySender) {
	if s == nil {
		return
	}
	s.groupMessages = messenger
}

func (s *Service) Enabled() bool {
	return s != nil && s.config.Enabled && s.agent != nil && s.commands != nil && s.policy != nil && s.contextBuilder != nil && s.logs != nil
}

func (s *Service) AssistantUUID() string {
	if s == nil {
		return ""
	}

	return strings.TrimSpace(s.config.AssistantUUID)
}

func (s *Service) HandleDirectMessage(ctx context.Context, message *model.Message) error {
	if !s.Enabled() || message == nil {
		return nil
	}

	if message.TargetType != model.MessageTargetDirect {
		return nil
	}

	assistantUUID := s.AssistantUUID()
	if assistantUUID == "" {
		return nil
	}
	if strings.TrimSpace(message.TargetUUID) != assistantUUID {
		return nil
	}
	if strings.TrimSpace(message.SenderUUID) == assistantUUID {
		return nil
	}

	startedAt := time.Now()
	started, err := s.logs.Begin(&model.AICallLog{
		TriggerMessageUUID: strings.TrimSpace(message.UUID),
		ConversationKey:    strings.TrimSpace(message.ConversationKey),
		UserUUID:           strings.TrimSpace(message.SenderUUID),
		AssistantUUID:      assistantUUID,
		Provider:           strings.TrimSpace(s.config.Provider),
		Model:              strings.TrimSpace(s.config.Model),
		Status:             model.AICallStatusPending,
	})
	if err != nil {
		return err
	}
	if !started {
		return nil
	}

	var policyExecution *application.AgentPolicyExecutionV1
	markFailed := func(err error) error {
		if policyExecution != nil {
			_ = s.policy.Fail(ctx, *policyExecution)
		}
		latencyMS := time.Since(startedAt).Milliseconds()
		_ = s.logs.MarkFailed(message.UUID, trimError(err), latencyMS)
		return err
	}

	ids := correlation.FromContext(ctx)
	policyExecution, err = s.policy.Start(ctx, application.AgentExecutionPolicyStartV1{
		TenantID: defaultAgentTenantID, PrincipalUUID: message.SenderUUID, AgentUUID: assistantUUID,
		DelegatedByUUID: message.SenderUUID, TriggerType: "message.direct.created", TriggerRef: message.UUID,
		RequestID: ids.RequestID, TraceID: ids.TraceID, EventID: ids.EventID,
	})
	if err != nil {
		return markFailed(err)
	}
	invocation := policyExecution.Invocation
	execution := newExecutionContext(ExecutionContext{
		TenantID:           defaultAgentTenantID,
		PrincipalUserUUID:  message.SenderUUID,
		AgentUUID:          assistantUUID,
		DelegatedByUUID:    message.SenderUUID,
		TriggerMessageUUID: message.UUID,
		ConversationKey:    message.ConversationKey,
		RequestID:          ids.RequestID,
		TraceID:            ids.TraceID,
		EventID:            ids.EventID,
	}, invocation.Permissions, invocation.ApprovedCapabilities, invocation.ResourceScopes)
	runCtx := withExecutionContext(ctx, execution)
	runCtx = eventlineage.AgentAction(runCtx, assistantUUID, policyExecution.TaskUUID, ids.EventID)

	conversationContext, err := s.contextBuilder.BuildDirectContext(runCtx, message.SenderUUID, assistantUUID)
	if err != nil {
		return markFailed(err)
	}

	runCtx = withToolExecutionState(runCtx, &toolExecutionState{})
	reply, err := s.agent.Reply(runCtx, conversationContext.Messages)
	if err != nil {
		return markFailed(err)
	}

	responseMessage := latestToolSentMessage(runCtx)
	if responseMessage == nil {
		content := strings.TrimSpace(reply.Content)
		if content == "" {
			return markFailed(ErrAIEmptyResponse)
		}

		responseMessage, err = s.commands.SendMessage(runCtx, application.AgentMessageCommandV1{
			CommandID:  "reply:" + strings.TrimSpace(message.UUID),
			Kind:       application.AgentMessageCommandAssistantReplyV1,
			Invocation: execution.invocationV1(),
			Content:    content,
		})
		if err != nil {
			return markFailed(err)
		}
	}

	usage := extractUsage(reply)
	latencyMS := time.Since(startedAt).Milliseconds()
	if err := s.policy.Complete(ctx, *policyExecution); err != nil {
		return markFailed(err)
	}
	policyExecution = nil
	if err := s.logs.MarkSucceeded(
		message.UUID,
		responseMessage.UUID,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
		latencyMS,
	); err != nil {
		return err
	}

	return nil
}

func (s *Service) HandleGroupMessage(ctx context.Context, message *model.Message) error {
	if !s.Enabled() || message == nil || s.groupMessages == nil {
		return nil
	}
	if message.TargetType != model.MessageTargetGroup {
		return nil
	}
	if message.MessageType != model.MessageTypeText {
		return nil
	}

	assistantUUID := s.AssistantUUID()
	if assistantUUID == "" {
		return nil
	}
	if strings.TrimSpace(message.SenderUUID) == assistantUUID {
		return nil
	}
	groupUUID := strings.TrimSpace(message.TargetUUID)
	if groupUUID == "" {
		return nil
	}
	if !DetectAssistantMention(message.Content, s.config.AssistantNickname, assistantUUID) {
		return nil
	}

	startedAt := time.Now()
	started, err := s.logs.Begin(&model.AICallLog{
		TriggerMessageUUID: strings.TrimSpace(message.UUID),
		ConversationKey:    strings.TrimSpace(message.ConversationKey),
		UserUUID:           strings.TrimSpace(message.SenderUUID),
		AssistantUUID:      assistantUUID,
		Provider:           strings.TrimSpace(s.config.Provider),
		Model:              strings.TrimSpace(s.config.Model),
		Status:             model.AICallStatusPending,
	})
	if err != nil {
		return err
	}
	if !started {
		return nil
	}

	var policyExecution *application.AgentPolicyExecutionV1
	markFailed := func(err error) error {
		if policyExecution != nil {
			_ = s.policy.Fail(ctx, *policyExecution)
		}
		latencyMS := time.Since(startedAt).Milliseconds()
		_ = s.logs.MarkFailed(message.UUID, trimError(err), latencyMS)
		return err
	}

	ids := correlation.FromContext(ctx)
	policyExecution, err = s.policy.Start(ctx, application.AgentExecutionPolicyStartV1{
		TenantID: defaultAgentTenantID, PrincipalUUID: message.SenderUUID, AgentUUID: assistantUUID,
		DelegatedByUUID: message.SenderUUID, TriggerType: "message.group.created", TriggerRef: message.UUID,
		RequestID: ids.RequestID, TraceID: ids.TraceID, EventID: ids.EventID,
	})
	if err != nil {
		return markFailed(err)
	}
	invocation := policyExecution.Invocation
	conversationKey := strings.TrimSpace(message.ConversationKey)
	if conversationKey == "" {
		conversationKey = model.GroupConversationKey(groupUUID)
	}
	execution := newExecutionContext(ExecutionContext{
		TenantID:           defaultAgentTenantID,
		PrincipalUserUUID:  message.SenderUUID,
		AgentUUID:          assistantUUID,
		DelegatedByUUID:    message.SenderUUID,
		TriggerMessageUUID: message.UUID,
		ConversationKey:    conversationKey,
		RequestID:          ids.RequestID,
		TraceID:            ids.TraceID,
		EventID:            ids.EventID,
	}, invocation.Permissions, invocation.ApprovedCapabilities, invocation.ResourceScopes)
	runCtx := withExecutionContext(ctx, execution)
	runCtx = eventlineage.AgentAction(runCtx, assistantUUID, policyExecution.TaskUUID, ids.EventID)

	conversationContext, err := s.contextBuilder.BuildGroupContext(runCtx, message.SenderUUID, assistantUUID, groupUUID)
	if err != nil {
		return markFailed(err)
	}

	runCtx = withToolExecutionState(runCtx, &toolExecutionState{})
	reply, err := s.agent.Reply(runCtx, conversationContext.Messages)
	if err != nil {
		return markFailed(err)
	}

	content := strings.TrimSpace(reply.Content)
	if content == "" {
		if toolMessage := latestToolSentMessage(runCtx); toolMessage != nil {
			content = strings.TrimSpace(toolMessage.Content)
		}
	}
	if content == "" {
		return markFailed(ErrAIEmptyResponse)
	}

	responseMessage, _, err := s.groupMessages.SendGroupMessage(
		assistantUUID,
		groupUUID,
		content,
		"reply:"+strings.TrimSpace(message.UUID),
	)
	if err != nil {
		return markFailed(err)
	}
	if responseMessage == nil || strings.TrimSpace(responseMessage.UUID) == "" {
		return markFailed(ErrAIEmptyResponse)
	}

	usage := extractUsage(reply)
	latencyMS := time.Since(startedAt).Milliseconds()
	if err := s.policy.Complete(ctx, *policyExecution); err != nil {
		return markFailed(err)
	}
	policyExecution = nil
	if err := s.logs.MarkSucceeded(
		message.UUID,
		responseMessage.UUID,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
		latencyMS,
	); err != nil {
		return err
	}

	return nil
}

type tokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func extractUsage(message *schema.Message) tokenUsage {
	if message == nil || message.ResponseMeta == nil || message.ResponseMeta.Usage == nil {
		return tokenUsage{}
	}

	return tokenUsage{
		PromptTokens:     message.ResponseMeta.Usage.PromptTokens,
		CompletionTokens: message.ResponseMeta.Usage.CompletionTokens,
		TotalTokens:      message.ResponseMeta.Usage.TotalTokens,
	}
}

func trimError(err error) string {
	if err == nil {
		return ""
	}

	message := strings.TrimSpace(err.Error())
	if len(message) <= 500 {
		return message
	}

	return message[:500]
}
