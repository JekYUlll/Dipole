package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

var _ application.ConversationStore = (*ConversationRepository)(nil)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository() *ConversationRepository {
	return &ConversationRepository{}
}

func NewConversationRepositoryWithDB(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error {
	return r.upsertMessage(userUUID, model.MessageTargetDirect, targetUUID, message, unreadIncrement, "direct")
}

func (r *ConversationRepository) UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error {
	return r.upsertMessage(userUUID, model.MessageTargetGroup, groupUUID, message, unreadIncrement, "group")
}

func (r *ConversationRepository) upsertMessage(userUUID string, targetType int8, targetUUID string, message *model.Message, unreadIncrement int, kind string) error {
	if message == nil {
		return fmt.Errorf("upsert %s conversation: message is required", kind)
	}
	conversation := &model.Conversation{
		UserUUID:              userUUID,
		TargetType:            targetType,
		TargetUUID:            targetUUID,
		ConversationKey:       message.ConversationKey,
		LastMessageUUID:       message.UUID,
		LastMessageType:       message.MessageType,
		LastMessagePreview:    model.BuildMessagePreview(message),
		LastMessageAt:         message.SentAt,
		LastMessageSenderUUID: message.SenderUUID,
		UnreadCount:           unreadIncrement,
	}
	unreadValue := any(gorm.Expr(
		"CASE WHEN last_message_uuid <> ? THEN 0 ELSE unread_count END",
		conversation.LastMessageUUID,
	))
	if unreadIncrement > 0 {
		unreadValue = gorm.Expr(
			"CASE WHEN last_message_uuid <> ? THEN unread_count + ? ELSE unread_count END",
			conversation.LastMessageUUID,
			unreadIncrement,
		)
	}
	updates := clause.Set{
		{Column: clause.Column{Name: "unread_count"}, Value: unreadValue},
		{Column: clause.Column{Name: "target_type"}, Value: conversation.TargetType},
		{Column: clause.Column{Name: "target_uuid"}, Value: conversation.TargetUUID},
		{Column: clause.Column{Name: "last_message_uuid"}, Value: conversation.LastMessageUUID},
		{Column: clause.Column{Name: "last_message_type"}, Value: conversation.LastMessageType},
		{Column: clause.Column{Name: "last_message_preview"}, Value: conversation.LastMessagePreview},
		{Column: clause.Column{Name: "last_message_at"}, Value: conversation.LastMessageAt},
		{Column: clause.Column{Name: "last_message_sender_uuid"}, Value: conversation.LastMessageSenderUUID},
		{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("CURRENT_TIMESTAMP(3)")},
	}
	if err := r.database().Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_uuid"},
			{Name: "conversation_key"},
		},
		DoUpdates: updates,
	}).Create(conversation).Error; err != nil {
		return fmt.Errorf("upsert %s conversation: %w", kind, err)
	}
	return nil
}

func (r *ConversationRepository) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	var conversations []*model.Conversation
	if err := r.database().Where("user_uuid = ?", userUUID).
		Order("last_message_at DESC").
		Limit(limit).
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by user uuid: %w", err)
	}
	return conversations, nil
}

func (r *ConversationRepository) GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error) {
	var conversation model.Conversation
	if err := r.database().Where("user_uuid = ? AND conversation_key = ?", userUUID, conversationKey).
		First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation by user and key: %w", err)
	}
	return &conversation, nil
}

func (r *ConversationRepository) InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error {
	conversation := &model.Conversation{
		UserUUID:        userUUID,
		TargetType:      model.MessageTargetGroup,
		TargetUUID:      groupUUID,
		ConversationKey: conversationKey,
		LastMessageAt:   createdAt,
	}
	if err := r.database().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_uuid"}, {Name: "conversation_key"}},
		DoNothing: true,
	}).Create(conversation).Error; err != nil {
		return fmt.Errorf("init group conversation: %w", err)
	}
	return nil
}

func (r *ConversationRepository) UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error {
	if err := r.database().Model(&model.Conversation{}).
		Where("user_uuid = ? AND conversation_key = ?", userUUID, conversationKey).
		Update("remark", remark).Error; err != nil {
		return fmt.Errorf("update conversation remark: %w", err)
	}
	return nil
}

func (r *ConversationRepository) ClearUnreadByConversationKey(userUUID, conversationKey string) error {
	if err := r.database().Model(&model.Conversation{}).
		Where("user_uuid = ? AND conversation_key = ?", userUUID, conversationKey).
		Update("unread_count", 0).Error; err != nil {
		return fmt.Errorf("clear conversation unread count: %w", err)
	}
	return nil
}

func (r *ConversationRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
