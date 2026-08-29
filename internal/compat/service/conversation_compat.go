package service

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
)

var (
	ErrConversationTargetRequired   = coreconversation.ErrConversationTargetRequired
	ErrConversationTargetNotFound   = coreconversation.ErrConversationTargetNotFound
	ErrConversationPermissionDenied = coreconversation.ErrConversationPermissionDenied
	ErrConversationRemarkTooLong    = coreconversation.ErrConversationRemarkTooLong
)

type ConversationView = coreconversation.ConversationView
type ConversationReadReceipt = coreconversation.ConversationReadReceipt
type ConversationService = coreconversation.ConversationService

func NewConversationService(repo interface {
	UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error
	UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error
	InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error
	ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error)
	GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error)
	MarkReadThroughByConversationKey(userUUID, conversationKey string, readThroughSeq uint64) error
	UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error
}, userFinder interface {
	GetByUUID(uuid string) (*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}, groupRepo interface {
	GetByUUID(groupUUID string) (*model.Group, error)
	ListMembers(groupUUID string) ([]*model.GroupMember, error)
	GetMember(groupUUID, userUUID string) (*model.GroupMember, error)
}, notifier interface {
	NotifyDirectRead(receipt coreconversation.ConversationReadReceipt)
}, events application.EventPublisher) *ConversationService {
	return coreconversation.NewConversationService(repo, userFinder, groupRepo, notifier, events)
}
