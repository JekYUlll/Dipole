package repository

import (
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
)

type MessageRepository struct{}

func NewMessageRepository() *MessageRepository {
	return &MessageRepository{}
}

// TODO: 为消息表接入自动分表策略，每 10 万条消息滚动到下一张物理表。
// 分表路由应收口在 repository / storage 层，避免 service 和 handler 直接感知表名。
func (r *MessageRepository) Create(message *model.Message) error {
	if err := store.DB.Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	return nil
}

func (r *MessageRepository) StoreWithOutbox(message *model.Message, event *model.OutboxEvent) error {
	if store.DB == nil {
		return fmt.Errorf("store message with outbox: mysql not initialized")
	}

	outboxRepo := NewOutboxRepository()
	if err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return fmt.Errorf("create message: %w", err)
		}
		if err := outboxRepo.Enqueue(tx, event); err != nil {
			return fmt.Errorf("enqueue outbox event: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("store message with outbox: %w", err)
	}

	return nil
}

func (r *MessageRepository) EnsureOutbox(event *model.OutboxEvent) error {
	if event == nil {
		return nil
	}

	if err := NewOutboxRepository().Enqueue(nil, event); err != nil {
		return fmt.Errorf("ensure outbox event: %w", err)
	}

	return nil
}

func (r *MessageRepository) GetByUUID(uuid string) (*model.Message, error) {
	var message model.Message
	if err := store.DB.Where("uuid = ?", uuid).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get message by uuid: %w", err)
	}

	return &message, nil
}

func (r *MessageRepository) GetBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.Message, error) {
	var message model.Message
	if err := store.DB.Where("sender_uuid = ? AND client_message_id = ?", senderUUID, clientMessageID).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get message by sender and client message id: %w", err)
	}

	return &message, nil
}

func (r *MessageRepository) HasConversationMessages(conversationKey string) (bool, error) {
	var count int64
	if err := store.DB.Model(&model.Message{}).Where("conversation_key = ?", conversationKey).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check messages by conversation key: %w", err)
	}

	return count > 0, nil
}

func (r *MessageRepository) ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error) {
	query := store.DB.Model(&model.Message{}).Where("conversation_key = ?", conversationKey)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}

	var messages []*model.Message
	if err := query.Order("id DESC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages by conversation key: %w", err)
	}

	reverseMessages(messages)
	return messages, nil
}

func (r *MessageRepository) ListByConversationKeyAfter(conversationKey string, afterID uint, limit int) ([]*model.Message, error) {
	query := store.DB.Model(&model.Message{}).Where("conversation_key = ?", conversationKey)
	if afterID > 0 {
		query = query.Where("id > ?", afterID)
	}

	var messages []*model.Message
	if err := query.Order("id ASC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages by conversation key after id: %w", err)
	}

	return messages, nil
}

func (r *MessageRepository) ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	query := store.DB.Model(&model.Message{}).Where("messages.id > ?", afterID).Where(
		"("+
			"(messages.target_type = ? AND messages.target_uuid = ?)"+
			" OR "+
			"(messages.target_type = ? AND messages.sender_uuid <> ? AND EXISTS ("+
			"SELECT 1 FROM group_members gm "+
			"JOIN `groups` g ON g.uuid = gm.group_uuid "+
			"WHERE gm.group_uuid = messages.target_uuid "+
			"AND gm.user_uuid = ? "+
			"AND g.status IN (?, ?)"+
			"))"+
			")",
		model.MessageTargetDirect,
		userUUID,
		model.MessageTargetGroup,
		userUUID,
		userUUID,
		model.GroupStatusNormal,
		model.GroupStatusDismissed,
	)

	var messages []*model.Message
	if err := query.Order("messages.id ASC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list offline messages by user uuid: %w", err)
	}

	return messages, nil
}

func (r *MessageRepository) FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error) {
	var message model.Message
	if err := store.DB.Model(&model.Message{}).
		Where("file_id = ? AND message_type = ?", fileUUID, model.MessageTypeFile).
		Where(
			"("+
				"(target_type = ? AND (sender_uuid = ? OR target_uuid = ?))"+
				" OR "+
				"(target_type = ? AND EXISTS ("+
				"SELECT 1 FROM group_members gm "+
				"JOIN `groups` g ON g.uuid = gm.group_uuid "+
				"WHERE gm.group_uuid = messages.target_uuid "+
				"AND gm.user_uuid = ? "+
				"AND g.status IN (?, ?)"+
				"))"+
				")",
			model.MessageTargetDirect,
			userUUID,
			userUUID,
			model.MessageTargetGroup,
			userUUID,
			model.GroupStatusNormal,
			model.GroupStatusDismissed,
		).
		Order("sent_at DESC").
		Order("id DESC").
		First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find latest accessible file message: %w", err)
	}

	return &message, nil
}

func FileMessageExpired(message *model.Message, now time.Time) bool {
	if message == nil || message.FileExpiresAt == nil {
		return false
	}
	return !message.FileExpiresAt.After(now)
}

func reverseMessages(messages []*model.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}
