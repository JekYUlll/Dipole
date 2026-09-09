package bootstrap

import (
	"context"
	"errors"
	"sync"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
)

// lazyCoreMessageSender avoids a Core/Message health-check cycle during
// startup. The first Core-owned system event establishes the RPC connection.
type lazyCoreMessageSender struct {
	cfg config.InternalRPC

	mu     sync.Mutex
	client *messagegrpc.Client
	conn   *grpc.ClientConn
}

func newLazyCoreMessageSender(cfg config.InternalRPC) *lazyCoreMessageSender {
	return &lazyCoreMessageSender{cfg: cfg}
}

func (s *lazyCoreMessageSender) SendSystemDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.SendSystemDirectMessage(senderUUID, targetUUID, content)
}

func (s *lazyCoreMessageSender) SendSystemGroupMessage(groupUUID, content string) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	return client.SendSystemGroupMessage(groupUUID, content)
}

func (s *lazyCoreMessageSender) SendAssistantTextMessageContext(ctx context.Context, assistantUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.SendAssistantTextMessageContext(ctx, assistantUUID, targetUUID, content, clientMessageID)
}

func (s *lazyCoreMessageSender) SendSystemDirectMessageCommandContext(ctx context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.SendSystemDirectMessageContext(ctx, senderUUID, targetUUID, content, clientMessageID)
}

func (s *lazyCoreMessageSender) SendAssistantGroupMessageContext(ctx context.Context, assistantUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, nil, err
	}
	return client.SendAssistantGroupMessageContext(ctx, assistantUUID, groupUUID, content, clientMessageID)
}

func (s *lazyCoreMessageSender) GetMessageCommandReceiptContext(ctx context.Context, senderUUID, clientMessageID string) (*application.MessageCommandReceipt, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetMessageCommandReceiptContext(ctx, senderUUID, clientMessageID)
}

func (s *lazyCoreMessageSender) GetMessageCommandReceipt(senderUUID, clientMessageID string) (*application.MessageCommandReceipt, error) {
	return s.GetMessageCommandReceiptContext(context.Background(), senderUUID, clientMessageID)
}

func (s *lazyCoreMessageSender) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.ListDirectMessages(currentUserUUID, targetUUID, beforeID, limit)
}

func (s *lazyCoreMessageSender) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return client.ListGroupMessages(currentUserUUID, groupUUID, beforeID, limit)
}

func (s *lazyCoreMessageSender) getClient() (*messagegrpc.Client, error) {
	if s == nil {
		return nil, errors.New("Core system message sender is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, conn, err := DialCoreMessageApplication(context.Background(), s.cfg)
	if err != nil {
		return nil, err
	}
	s.client = client
	s.conn = conn
	return client, nil
}

func (s *lazyCoreMessageSender) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.client = nil
	s.conn = nil
	return err
}
