package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ollamaModel "github.com/cloudwego/eino-ext/components/model/ollama"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	einoModel "github.com/cloudwego/eino/components/model"

	"github.com/JekYUlll/Dipole/internal/config"
)

type ChatModelFactory interface {
	Create(ctx context.Context, provider config.AIProvider) (einoModel.BaseChatModel, error)
}

type DefaultChatModelFactory struct{}

func NewChatModelFactory() ChatModelFactory {
	return DefaultChatModelFactory{}
}

func (DefaultChatModelFactory) Create(ctx context.Context, provider config.AIProvider) (einoModel.BaseChatModel, error) {
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second

	switch strings.ToLower(strings.TrimSpace(provider.Name)) {
	case "", "openai":
		if strings.TrimSpace(provider.APIKey) == "" {
			return nil, errors.New("ai api key is empty")
		}
		return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
			APIKey:  strings.TrimSpace(provider.APIKey),
			BaseURL: strings.TrimSpace(provider.BaseURL),
			Model:   strings.TrimSpace(provider.Model),
			Timeout: timeout,
		})
	case "ollama":
		baseURL := strings.TrimSpace(provider.BaseURL)
		if baseURL == "" {
			baseURL = "http://127.0.0.1:11434"
		}
		return ollamaModel.NewChatModel(ctx, &ollamaModel.ChatModelConfig{
			BaseURL: baseURL,
			Model:   strings.TrimSpace(provider.Model),
			Timeout: timeout,
		})
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", provider.Name)
	}
}
