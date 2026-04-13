package ai

import (
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubAIMessageReader struct {
	messages []*model.Message
	err      error
}

func (s *stubAIMessageReader) ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.messages, nil
}

type stubAIUserReader struct {
	users map[string]*model.User
	err   error
}

func (s *stubAIUserReader) GetByUUID(uuid string) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.users[uuid], nil
}

func TestContextBuilderBuildDirectContext(t *testing.T) {
	t.Parallel()

	builder := NewContextBuilder(
		&stubAIMessageReader{
			messages: []*model.Message{
				{
					UUID:        "M1",
					SenderUUID:  "U100",
					MessageType: model.MessageTypeText,
					Content:     "hello",
				},
				{
					UUID:        "M2",
					SenderUUID:  "UAI",
					MessageType: model.MessageTypeAIText,
					Content:     "hi, how can I help?",
				},
				{
					UUID:            "M3",
					SenderUUID:      "U100",
					MessageType:     model.MessageTypeFile,
					FileName:        "hello.txt",
					FileSize:        128,
					FileContentType: "text/plain",
					FileURL:         "http://example.com/hello.txt",
				},
			},
		},
		&stubAIUserReader{
			users: map[string]*model.User{
				"U100": {UUID: "U100", UserType: model.UserTypeNormal},
				"UAI":  {UUID: "UAI", UserType: model.UserTypeAssistant},
			},
		},
		12,
	)

	context, err := builder.BuildDirectContext("U100", "UAI")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(context.Messages) != 3 {
		t.Fatalf("expected 3 context messages, got %d", len(context.Messages))
	}
	if context.Messages[0].Role != schema.User {
		t.Fatalf("expected first message to be user, got %s", context.Messages[0].Role)
	}
	if context.Messages[1].Role != schema.Assistant {
		t.Fatalf("expected second message to be assistant, got %s", context.Messages[1].Role)
	}
	if context.Messages[2].Role != schema.User {
		t.Fatalf("expected file message to remain user-side context, got %s", context.Messages[2].Role)
	}
	if context.Messages[2].Content == "" {
		t.Fatalf("expected rendered file content, got empty string")
	}
}
