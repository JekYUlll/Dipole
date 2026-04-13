package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

type MessageRepository struct{}

func NewMessageRepository() *MessageRepository {
	return &MessageRepository{}
}

func (r *MessageRepository) Create(message *model.Message) error {
	if err := store.DB.Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	return nil
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

func reverseMessages(messages []*model.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}
