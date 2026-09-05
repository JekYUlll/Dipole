package bootstrap

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
)

type coreMessageHistoryClient interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
}

type coreMessageHistoryDialer func(context.Context, config.InternalRPC) (coreMessageHistoryClient, io.Closer, error)

// lazyCoreMessageSender avoids a Core/Message health-check cycle during
// startup. The first Core-owned system event establishes the RPC connection.
type lazyCoreMessageSender struct {
	cfg config.InternalRPC

	mu     sync.Mutex
	client *messagegrpc.Client
	conn   *grpc.ClientConn
}

var _ application.MessageCommandReceiptQuery = (*lazyCoreMessageSender)(nil)

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

func (s *lazyCoreMessageSender) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	return s.SendGroupMessageContext(context.Background(), senderUUID, groupUUID, content, clientMessageID)
}

func (s *lazyCoreMessageSender) SendGroupMessageContext(ctx context.Context, senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, nil, err
	}
	return client.SendGroupMessageContext(ctx, senderUUID, groupUUID, content, clientMessageID)
}

// These methods keep Agent writes on the Message service in standalone Core
// mode while preserving the stable command key used for replay recovery.
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
	return client.SendSystemDirectMessageCommandContext(ctx, senderUUID, targetUUID, content, clientMessageID)
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

func (s *lazyCoreMessageSender) getClient() (*messagegrpc.Client, error) {
	if s == nil {
		return nil, errors.New("Core system message sender is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, conn, err := dialCoreMessageApplication(context.Background(), s.cfg)
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

// lazyCoreMessageReader keeps Agent Capability reads on the Message service
// when standalone Core owns no local message repository.
type lazyCoreMessageReader struct {
	cfg  config.InternalRPC
	dial coreMessageHistoryDialer

	mu     sync.Mutex
	client coreMessageHistoryClient
	conn   io.Closer
}

func newLazyCoreMessageReader(cfg config.InternalRPC) *lazyCoreMessageReader {
	return &lazyCoreMessageReader{cfg: cfg, dial: dialCoreMessageHistory}
}

func (r *lazyCoreMessageReader) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	client, err := r.getClient()
	if err != nil {
		return nil, err
	}
	return client.ListDirectMessages(currentUserUUID, targetUUID, beforeID, limit)
}

func (r *lazyCoreMessageReader) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	client, err := r.getClient()
	if err != nil {
		return nil, err
	}
	return client.ListGroupMessages(currentUserUUID, groupUUID, beforeID, limit)
}

func (r *lazyCoreMessageReader) getClient() (coreMessageHistoryClient, error) {
	if r == nil {
		return nil, errors.New("Core Message reader is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return r.client, nil
	}
	client, conn, err := r.dial(context.Background(), r.cfg)
	if err != nil {
		return nil, err
	}
	r.client = client
	r.conn = conn
	return client, nil
}

func (r *lazyCoreMessageReader) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return nil
	}
	err := r.conn.Close()
	r.client = nil
	r.conn = nil
	return err
}

func dialCoreMessageHistory(ctx context.Context, cfg config.InternalRPC) (coreMessageHistoryClient, io.Closer, error) {
	client, conn, err := dialCoreMessageApplication(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, conn, nil
}
