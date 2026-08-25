// Package application defines transport-neutral use-case contracts.
package application

import (
	"context"

	"github.com/JekYUlll/Dipole/internal/model"
)

type MessageCommand interface {
	SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error)
	SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error)
	SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error)
}

type MessageQuery interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error)
	ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error)
}

type MessageApplication interface {
	MessageCommand
	MessageQuery
}

type SyncPage struct {
	Items   []*model.SyncMessage
	NextSeq uint64
	HasMore bool
}

type SyncApplication interface {
	List(userUUID string, afterSeq uint64, limit int) (*SyncPage, error)
}

type EventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}
