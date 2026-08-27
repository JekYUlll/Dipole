package ai

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

type stubContextBuilder struct {
	context *ConversationContext
	err     error
}

func (s *stubContextBuilder) BuildDirectContext(context.Context, string, string) (*ConversationContext, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.context, nil
}

type stubCallLogRepository struct {
	beginReturn bool
	beginLog    *model.AICallLog
	successArgs []any
	failedArgs  []any
}

func (s *stubCallLogRepository) Begin(log *model.AICallLog) (bool, error) {
	s.beginLog = log
	return s.beginReturn, nil
}

func (s *stubCallLogRepository) MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error {
	s.successArgs = []any{triggerMessageUUID, responseMessageUUID, promptTokens, completionTokens, totalTokens}
	return nil
}

func (s *stubCallLogRepository) MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error {
	s.failedArgs = []any{triggerMessageUUID, errorMessage}
	return nil
}

type stubAgentCommands struct {
	command application.AgentMessageCommandV1
	ids     correlation.IDs
	message *model.Message
	err     error
}

func (s *stubAgentCommands) SendMessage(ctx context.Context, command application.AgentMessageCommandV1) (*model.Message, error) {
	s.command = command
	s.ids = correlation.FromContext(ctx)
	if s.err != nil {
		return nil, s.err
	}

	return s.message, nil
}

type stubAgent struct {
	reply *schema.Message
	err   error
	runFn func(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}

func (s *stubAgent) Reply(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	if s.runFn != nil {
		return s.runFn(ctx, messages)
	}
	if s.err != nil {
		return nil, s.err
	}

	return s.reply, nil
}

func TestServiceHandleDirectMessageSuccess(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	commands := &stubAgentCommands{
		message: &model.Message{UUID: "MAI200"},
	}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			Provider:      "openai",
			Model:         "gpt-test",
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			},
		},
		logs:     logs,
		commands: commands,
		agent: &stubAgent{
			reply: &schema.Message{
				Role:    schema.Assistant,
				Content: "ai response",
				ResponseMeta: &schema.ResponseMeta{
					Usage: &schema.TokenUsage{
						PromptTokens:     10,
						CompletionTokens: 5,
						TotalTokens:      15,
					},
				},
			},
		},
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog == nil || logs.beginLog.TriggerMessageUUID != "M100" {
		t.Fatalf("expected ai call log begin with trigger M100, got %+v", logs.beginLog)
	}
	if commands.command.Invocation.AgentUUID != "UAI" || commands.command.Invocation.PrincipalUUID != "U100" {
		t.Fatalf("unexpected command identity: %+v", commands.command)
	}
	if commands.command.CommandID != "reply:M100" || commands.command.Kind != application.AgentMessageCommandAssistantReplyV1 || commands.command.Content != "ai response" {
		t.Fatalf("unexpected reply command: %+v", commands.command)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
	}
}

func TestServiceDerivesTrustedExecutionContextFromTrigger(t *testing.T) {
	t.Parallel()

	var captured ExecutionContext
	service := &Service{
		config: config.AI{Enabled: true, AssistantUUID: "UAI"},
		contextBuilder: &stubContextBuilder{context: &ConversationContext{
			Messages: []*schema.Message{schema.UserMessage("hello")},
		}},
		logs:     &stubCallLogRepository{beginReturn: true},
		commands: &stubAgentCommands{message: &model.Message{UUID: "M-REPLY"}},
		agent: &stubAgent{runFn: func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
			var err error
			captured, err = requireExecutionContext(ctx)
			if err != nil {
				return nil, err
			}
			if len(messages) != 1 || messages[0].Role != schema.User {
				t.Fatalf("execution identity leaked into model messages: %+v", messages)
			}
			return schema.AssistantMessage("reply", nil), nil
		}},
	}

	ctx := correlation.WithContext(context.Background(), correlation.IDs{
		RequestID: "REQ-1",
		TraceID:   "TRACE-1",
		EventID:   "EVENT-1",
	})
	message := &model.Message{
		UUID:            "M-TRIGGER",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	}
	if err := service.HandleDirectMessage(ctx, message); err != nil {
		t.Fatalf("handle direct message: %v", err)
	}

	want := ExecutionContext{
		TenantID:           defaultAgentTenantID,
		PrincipalUserUUID:  "U100",
		AgentUUID:          "UAI",
		DelegatedByUUID:    "U100",
		TriggerMessageUUID: "M-TRIGGER",
		ConversationKey:    message.ConversationKey,
		RequestID:          "REQ-1",
		TraceID:            "TRACE-1",
		EventID:            "EVENT-1",
	}
	want = newExecutionContext(want, embeddedAgentPermissionsV1(), nil)
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("execution context = %+v, want %+v", captured, want)
	}
}

func TestServiceHandleDirectMessageSkipsNonAssistantTarget(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{},
		logs:           logs,
		commands:       &stubAgentCommands{},
		agent:          &stubAgent{},
	}

	if err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:        "M100",
		SenderUUID:  "U100",
		TargetType:  model.MessageTargetDirect,
		TargetUUID:  "U200",
		MessageType: model.MessageTypeText,
		Content:     "hello",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if logs.beginLog != nil {
		t.Fatalf("expected ai pipeline to skip non-assistant message")
	}
}

func TestServiceHandleDirectMessageMarksFailure(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			},
		},
		logs:     logs,
		commands: &stubAgentCommands{},
		agent: &stubAgent{
			err: errors.New("llm timeout"),
		},
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if len(logs.failedArgs) == 0 {
		t.Fatalf("expected ai call failure log to be recorded")
	}
}

func TestServiceHandleDirectMessageUsesToolSentMessage(t *testing.T) {
	t.Parallel()

	logs := &stubCallLogRepository{beginReturn: true}
	toolMessage := &model.Message{
		UUID:        "MSYS100",
		MessageType: model.MessageTypeSystem,
	}
	commands := &stubAgentCommands{}
	service := &Service{
		config: config.AI{
			Enabled:       true,
			Provider:      "openai",
			Model:         "gpt-test",
			AssistantUUID: "UAI",
		},
		contextBuilder: &stubContextBuilder{
			context: &ConversationContext{
				Messages: []*schema.Message{schema.UserMessage("send a notice")},
			},
		},
		logs:     logs,
		commands: commands,
		agent: &stubAgent{
			runFn: func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
				recordToolSentMessage(ctx, toolMessage)
				return &schema.Message{
					Role:    schema.Assistant,
					Content: "",
				}, nil
			},
		},
	}

	err := service.HandleDirectMessage(context.Background(), &model.Message{
		UUID:            "M100",
		ConversationKey: model.DirectConversationKey("U100", "UAI"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "UAI",
		MessageType:     model.MessageTypeText,
		Content:         "send a notice",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if commands.command.Content != "" {
		t.Fatalf("expected no fallback assistant text command, got %q", commands.command.Content)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
	}
	if logs.successArgs[1] != "MSYS100" {
		t.Fatalf("expected tool-sent message uuid to be recorded, got %+v", logs.successArgs)
	}
}
