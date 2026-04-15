package ai

import (
	"context"
	"errors"
	"testing"

	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type stubBaseChatModel struct {
	generateFn func(ctx context.Context, input []*schema.Message, opts ...einoModel.Option) (*schema.Message, error)
	toolInfos  []*schema.ToolInfo
}

func (s *stubBaseChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einoModel.Option) (*schema.Message, error) {
	if s.generateFn == nil {
		return nil, errors.New("generateFn is nil")
	}

	return s.generateFn(ctx, input, opts...)
}

func (s *stubBaseChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einoModel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not implemented in test")
}

func (s *stubBaseChatModel) WithTools(tools []*schema.ToolInfo) (einoModel.ToolCallingChatModel, error) {
	s.toolInfos = tools
	return s, nil
}

func TestEinoChatAgentReply(t *testing.T) {
	t.Parallel()

	var captured []*schema.Message
	agent, err := NewAgent(context.Background(), &stubBaseChatModel{
		generateFn: func(ctx context.Context, input []*schema.Message, opts ...einoModel.Option) (*schema.Message, error) {
			captured = input
			return schema.AssistantMessage("hello from eino", nil), nil
		},
	}, "You are Dipole AI")
	if err != nil {
		t.Fatalf("expected no error creating agent, got %v", err)
	}

	reply, err := agent.Reply(context.Background(), []*schema.Message{
		schema.UserMessage("hi"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reply == nil || reply.Content != "hello from eino" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if len(captured) != 2 {
		t.Fatalf("expected instruction + one user message, got %d", len(captured))
	}
	if captured[0].Role != schema.System {
		t.Fatalf("expected first message to be system, got %s", captured[0].Role)
	}
}

func TestNewAgentAcceptsTools(t *testing.T) {
	t.Parallel()

	model := &stubBaseChatModel{
		generateFn: func(ctx context.Context, input []*schema.Message, opts ...einoModel.Option) (*schema.Message, error) {
			return schema.AssistantMessage("tool-enabled", nil), nil
		},
	}

	agent, err := NewAgent(
		context.Background(),
		model,
		"You are Dipole AI",
		NewUserProfileTool(&stubAIUserReader{}),
	)
	if err != nil {
		t.Fatalf("expected no error creating agent, got %v", err)
	}
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	if _, err := agent.Reply(context.Background(), []*schema.Message{
		schema.UserMessage("hello"),
	}); err != nil {
		t.Fatalf("expected no error running agent, got %v", err)
	}
}
