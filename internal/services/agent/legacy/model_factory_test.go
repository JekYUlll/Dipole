package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestDefaultChatModelFactoryRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	factory := NewChatModelFactory()
	_, err := factory.Create(context.Background(), config.AIProvider{
		Name:  "unknown",
		Model: "demo",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unsupported ai provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultChatModelFactoryRequiresOpenAIKey(t *testing.T) {
	t.Parallel()

	factory := NewChatModelFactory()
	_, err := factory.Create(context.Background(), config.AIProvider{
		Name:  "openai",
		Model: "gpt-4o-mini",
	})
	if err == nil {
		t.Fatal("expected error when api key is empty")
	}
	if !strings.Contains(err.Error(), "ai api key is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
