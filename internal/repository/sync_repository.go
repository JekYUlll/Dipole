package repository

import (
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
)

var _ application.SyncStore = (*SyncRepository)(nil)

type SyncRepository struct {
	db *gorm.DB
}

func NewSyncRepository() *SyncRepository {
	return &SyncRepository{}
}

func NewSyncRepositoryWithDB(db *gorm.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error) {
	if r.database() == nil {
		return nil, fmt.Errorf("list user sync inbox: mysql not initialized")
	}
	var inboxRows []*model.UserSyncInbox
	if err := r.database().Where("user_uuid = ? AND sync_seq > ?", strings.TrimSpace(userUUID), afterSeq).
		Order("sync_seq ASC").
		Limit(limit).
		Find(&inboxRows).Error; err != nil {
		return nil, fmt.Errorf("list user sync inbox: %w", err)
	}
	if len(inboxRows) == 0 {
		return []*model.SyncMessage{}, nil
	}

	messageUUIDs := make([]string, 0, len(inboxRows))
	for _, row := range inboxRows {
		messageUUIDs = append(messageUUIDs, row.MessageUUID)
	}
	var messages []*model.Message
	if err := r.database().Where("uuid IN ?", messageUUIDs).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages for sync inbox: %w", err)
	}
	messageByUUID := make(map[string]*model.Message, len(messages))
	for _, message := range messages {
		messageByUUID[message.UUID] = message
	}

	items := make([]*model.SyncMessage, 0, len(inboxRows))
	for _, row := range inboxRows {
		message := messageByUUID[row.MessageUUID]
		if message == nil {
			return nil, fmt.Errorf("sync inbox message %s is missing", row.MessageUUID)
		}
		items = append(items, &model.SyncMessage{
			SyncSeq:         row.SyncSeq,
			ConversationKey: row.ConversationKey,
			Message:         message,
		})
	}
	return items, nil
}

func (r *SyncRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
