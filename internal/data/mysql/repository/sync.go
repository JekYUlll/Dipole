package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.SyncStore = (*SyncRepository)(nil)

type SyncRepository struct{ queries *generated.Queries }

func NewSyncRepository(queries *generated.Queries) (*SyncRepository, error) {
	if queries == nil {
		return nil, fmt.Errorf("sync queries are required")
	}
	return &SyncRepository{queries: queries}, nil
}

func (r *SyncRepository) ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error) {
	ctx := context.Background()
	inbox, err := r.queries.ListUserSyncInboxAfter(ctx, generated.ListUserSyncInboxAfterParams{UserUuid: strings.TrimSpace(userUUID), SyncSeq: afterSeq, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	if len(inbox) == 0 {
		return []*model.SyncMessage{}, nil
	}
	ids := make([]string, 0, len(inbox))
	for _, row := range inbox {
		ids = append(ids, row.MessageUuid)
	}
	rows, err := r.queries.ListMessagesByUUIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Message, len(rows))
	for _, row := range rows {
		message := mapper.Message(row)
		byID[message.UUID] = message
	}
	items := make([]*model.SyncMessage, 0, len(inbox))
	for _, row := range inbox {
		message := byID[row.MessageUuid]
		if message == nil {
			return nil, fmt.Errorf("sync inbox message %s is missing", row.MessageUuid)
		}
		items = append(items, &model.SyncMessage{SyncSeq: row.SyncSeq, ConversationKey: row.ConversationKey, Message: message})
	}
	return items, nil
}
