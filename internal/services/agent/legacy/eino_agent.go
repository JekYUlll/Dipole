package ai

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/adk"
	einoModel "github.com/cloudwego/eino/components/model"
	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"strings"

	"github.com/JekYUlll/Dipole/internal/config"
)

var ErrAIEmptyResponse = errors.New("ai response is empty")

type Agent interface {
	Reply(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}

type EinoChatAgent struct {
	runner *adk.Runner
}

func NewConfiguredAgent(ctx context.Context, tools ...einoTool.BaseTool) (*EinoChatAgent, error) {
	cfg := config.AIConfig()
	model, err := NewChatModelFactory().Create(ctx, cfg.DefaultProvider())
	if err != nil {
		return nil, err
	}

	return NewAgent(ctx, model, cfg.SystemPrompt, tools...)
}

func NewAgent(ctx context.Context, chatModel einoModel.BaseChatModel, instruction string, tools ...einoTool.BaseTool) (*EinoChatAgent, error) {
	if chatModel == nil {
		return nil, errors.New("ai chat model is nil")
	}

	agentConfig := &adk.ChatModelAgentConfig{
		Name:        "dipole_ai_assistant",
		Description: "Dipole AI instant messaging assistant",
		Instruction: strings.TrimSpace(instruction),
		Model:       chatModel,
	}
	if len(tools) > 0 {
		agentConfig.ToolsConfig.Tools = tools
	}

	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("create eino chat model agent: %w", err)
	}

	return &EinoChatAgent{
		runner: adk.NewRunner(ctx, adk.RunnerConfig{
			Agent: agent,
		}),
	}, nil
}

func (a *EinoChatAgent) Reply(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	if a == nil || a.runner == nil {
		return nil, errors.New("ai runner is nil")
	}

	iter := a.runner.Run(ctx, messages)
	var reply *schema.Message
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return nil, event.Err
		}

		msg, _, err := adk.GetMessage(event)
		if err != nil {
			return nil, err
		}
		if msg == nil || msg.Role != schema.Assistant {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}

		reply = msg
	}

	if reply == nil {
		return nil, ErrAIEmptyResponse
	}

	return reply, nil
}
