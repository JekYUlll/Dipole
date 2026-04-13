package repository

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

type ConversationRepository struct{}

func NewConversationRepository() *ConversationRepository {
	return &ConversationRepository{}
}

func (r *ConversationRepository) UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error {
	conversation := &model.Conversation{
		UserUUID:           userUUID,
		TargetType:         model.MessageTargetDirect,
		TargetUUID:         targetUUID,
		ConversationKey:    message.ConversationKey,
		LastMessageUUID:    message.UUID,
		LastMessageType:    message.MessageType,
		LastMessagePreview: buildMessagePreview(message),
		LastMessageAt:      message.SentAt,
		UnreadCount:        unreadIncrement,
	}

	assignments := map[string]any{
		"target_type":          conversation.TargetType,
		"target_uuid":          conversation.TargetUUID,
		"last_message_uuid":    conversation.LastMessageUUID,
		"last_message_type":    conversation.LastMessageType,
		"last_message_preview": conversation.LastMessagePreview,
		"last_message_at":      conversation.LastMessageAt,
		"updated_at":           gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if unreadIncrement > 0 {
		assignments["unread_count"] = gorm.Expr(
			"CASE WHEN last_message_uuid <> ? THEN unread_count + ? ELSE unread_count END",
			conversation.LastMessageUUID,
			unreadIncrement,
		)
	} else {
		assignments["unread_count"] = gorm.Expr(
			"CASE WHEN last_message_uuid <> ? THEN 0 ELSE unread_count END",
			conversation.LastMessageUUID,
		)
	}

	if err := store.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_uuid"},
			{Name: "conversation_key"},
		},
		DoUpdates: clause.Assignments(assignments),
	}).Create(conversation).Error; err != nil {
		return fmt.Errorf("upsert direct conversation: %w", err)
	}

	return nil
}

func (r *ConversationRepository) UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error {
	conversation := &model.Conversation{
		UserUUID:           userUUID,
		TargetType:         model.MessageTargetGroup,
		TargetUUID:         groupUUID,
		ConversationKey:    message.ConversationKey,
		LastMessageUUID:    message.UUID,
		LastMessageType:    message.MessageType,
		LastMessagePreview: buildMessagePreview(message),
		LastMessageAt:      message.SentAt,
		UnreadCount:        unreadIncrement,
	}

	assignments := map[string]any{
		"target_type":          conversation.TargetType,
		"target_uuid":          conversation.TargetUUID,
		"last_message_uuid":    conversation.LastMessageUUID,
		"last_message_type":    conversation.LastMessageType,
		"last_message_preview": conversation.LastMessagePreview,
		"last_message_at":      conversation.LastMessageAt,
		"updated_at":           gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if unreadIncrement > 0 {
		assignments["unread_count"] = gorm.Expr(
			"CASE WHEN last_message_uuid <> ? THEN unread_count + ? ELSE unread_count END",
			conversation.LastMessageUUID,
			unreadIncrement,
		)
	} else {
		assignments["unread_count"] = gorm.Expr(
			"CASE WHEN last_message_uuid <> ? THEN 0 ELSE unread_count END",
			conversation.LastMessageUUID,
		)
	}

	if err := store.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_uuid"},
			{Name: "conversation_key"},
		},
		DoUpdates: clause.Assignments(assignments),
	}).Create(conversation).Error; err != nil {
		return fmt.Errorf("upsert group conversation: %w", err)
	}

	return nil
}

func (r *ConversationRepository) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	var conversations []*model.Conversation
	if err := store.DB.Where("user_uuid = ?", userUUID).
		Order("last_message_at DESC").
		Limit(limit).
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by user uuid: %w", err)
	}

	return conversations, nil
}

func (r *ConversationRepository) ClearUnreadByConversationKey(userUUID, conversationKey string) error {
	if err := store.DB.Model(&model.Conversation{}).
		Where("user_uuid = ? AND conversation_key = ?", userUUID, conversationKey).
		Update("unread_count", 0).Error; err != nil {
		return fmt.Errorf("clear conversation unread count: %w", err)
	}

	return nil
}

func buildMessagePreview(message *model.Message) string {
	switch message.MessageType {
	case model.MessageTypeText:
		runes := []rune(message.Content)
		if len(runes) <= 100 {
			return message.Content
		}
		return string(runes[:100])
	default:
		return "[unsupported]"
	}
}
