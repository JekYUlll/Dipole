package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubContextBuilder struct {
	context *ConversationContext
	err     error
}

func (s *stubContextBuilder) BuildDirectContext(userUUID, assistantUUID string) (*ConversationContext, error) {
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

type stubMessageSender struct {
	assistantUUID string
	targetUUID    string
	content       string
	message       *model.Message
	err           error
}

func (s *stubMessageSender) SendAssistantTextMessage(assistantUUID, targetUUID, content string) (*model.Message, error) {
	s.assistantUUID = assistantUUID
	s.targetUUID = targetUUID
	s.content = content
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
	sender := &stubMessageSender{
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
		logs:   logs,
		sender: sender,
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
	if sender.assistantUUID != "UAI" || sender.targetUUID != "U100" {
		t.Fatalf("unexpected sender args: %+v", sender)
	}
	if sender.content != "ai response" {
		t.Fatalf("expected ai response content, got %q", sender.content)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
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
		sender:         &stubMessageSender{},
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
		logs:   logs,
		sender: &stubMessageSender{},
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
	sender := &stubMessageSender{}
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
		logs:   logs,
		sender: sender,
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
	if sender.content != "" {
		t.Fatalf("expected no fallback assistant text send, got %q", sender.content)
	}
	if len(logs.successArgs) == 0 {
		t.Fatalf("expected ai call success log to be recorded")
	}
	if logs.successArgs[1] != "MSYS100" {
		t.Fatalf("expected tool-sent message uuid to be recorded, got %+v", logs.successArgs)
	}
}
