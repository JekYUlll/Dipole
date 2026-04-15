package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
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

type directContextBuilder interface {
	BuildDirectContext(userUUID, assistantUUID string) (*ConversationContext, error)
}

type messageSender interface {
	SendAssistantTextMessage(assistantUUID, targetUUID, content string) (*model.Message, error)
}

type Service struct {
	config         config.AI
	contextBuilder directContextBuilder
	logs           callLogRepository
	sender         messageSender
	agent          Agent
}

func NewService(builder directContextBuilder, logs callLogRepository, sender messageSender, agent Agent) *Service {
	return &Service{
		config:         config.AIConfig(),
		contextBuilder: builder,
		logs:           logs,
		sender:         sender,
		agent:          agent,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.config.Enabled && s.agent != nil && s.sender != nil && s.contextBuilder != nil && s.logs != nil
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

	markFailed := func(err error) error {
		latencyMS := time.Since(startedAt).Milliseconds()
		_ = s.logs.MarkFailed(message.UUID, trimError(err), latencyMS)
		return err
	}

	conversationContext, err := s.contextBuilder.BuildDirectContext(message.SenderUUID, assistantUUID)
	if err != nil {
		return markFailed(err)
	}

	runCtx := withToolExecutionState(ctx, &toolExecutionState{})
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

		responseMessage, err = s.sender.SendAssistantTextMessage(assistantUUID, message.SenderUUID, content)
		if err != nil {
			return markFailed(err)
		}
	}

	usage := extractUsage(reply)
	latencyMS := time.Since(startedAt).Milliseconds()
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
