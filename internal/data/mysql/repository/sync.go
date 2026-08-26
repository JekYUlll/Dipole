package repository

import (
	"context"
	"database/sql"
	"errors"
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

func (r *SyncRepository) GetDeviceCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	row, err := r.queries.GetDeviceSyncCheckpoint(context.Background(), generated.GetDeviceSyncCheckpointParams{
		UserUuid: strings.TrimSpace(userUUID), DeviceID: strings.TrimSpace(deviceID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.DeviceSyncCheckpoint{UserUUID: row.UserUuid, DeviceID: row.DeviceID, SyncSeq: row.SyncSeq, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *SyncRepository) GetLatestUserSyncSequence(userUUID string) (uint64, error) {
	sequence, err := r.queries.GetLatestUserSyncSequence(context.Background(), strings.TrimSpace(userUUID))
	if err != nil {
		return 0, err
	}
	if sequence < 0 {
		return 0, errors.New("latest user sync sequence is negative")
	}
	return uint64(sequence), nil
}

func (r *SyncRepository) AdvanceDeviceSyncCheckpoint(userUUID, deviceID string, syncSeq uint64) error {
	_, err := r.queries.AdvanceDeviceSyncCheckpoint(context.Background(), generated.AdvanceDeviceSyncCheckpointParams{
		UserUuid: strings.TrimSpace(userUUID), DeviceID: strings.TrimSpace(deviceID), SyncSeq: syncSeq,
	})
	return err
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
