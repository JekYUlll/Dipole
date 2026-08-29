package syncmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.SyncStore = (*SyncRepository)(nil)

type SyncRepository struct {
	queries  *generated.Queries
	hydrator application.SyncMessageHydrator
}

func NewSyncRepository(queries *generated.Queries) (*SyncRepository, error) {
	hydrator, err := NewMySQLSyncMessageHydrator(queries)
	if err != nil {
		return nil, err
	}
	return NewSyncRepositoryWithHydrator(queries, hydrator)
}

func NewSyncRepositoryWithHydrator(queries *generated.Queries, hydrator application.SyncMessageHydrator) (*SyncRepository, error) {
	if queries == nil {
		return nil, fmt.Errorf("sync queries are required")
	}
	if hydrator == nil {
		return nil, fmt.Errorf("Sync message hydrator is required")
	}
	return &SyncRepository{queries: queries, hydrator: hydrator}, nil
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

func (r *SyncRepository) ListGroupSyncCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	if len(groupUUIDs) == 0 {
		return []*model.GroupSyncCheckpoint{}, nil
	}
	rows, err := r.queries.ListGroupSyncCheckpoints(context.Background(), generated.ListGroupSyncCheckpointsParams{
		UserUuid: strings.TrimSpace(userUUID), DeviceID: strings.TrimSpace(deviceID), GroupUuids: groupUUIDs,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*model.GroupSyncCheckpoint, 0, len(rows))
	for _, row := range rows {
		result = append(result, &model.GroupSyncCheckpoint{GroupUUID: row.GroupUuid, LatestMessageSeq: row.LatestMessageSeq, LatestMessageUUID: row.LatestMessageUuid, PulledMessageSeq: row.PulledMessageSeq})
	}
	return result, nil
}

func (r *SyncRepository) GetGroupSyncState(groupUUID string) (*model.GroupSyncState, error) {
	row, err := r.queries.GetGroupSyncState(context.Background(), strings.TrimSpace(groupUUID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.GroupSyncState{GroupUUID: row.GroupUuid, LatestMessageSeq: row.LatestMessageSeq, LatestMessageUUID: row.LatestMessageUuid, UpdatedAt: row.UpdatedAt}, nil
}

func (r *SyncRepository) AdvanceDeviceGroupSyncCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) error {
	return r.queries.AdvanceDeviceGroupSyncCheckpoint(context.Background(), generated.AdvanceDeviceGroupSyncCheckpointParams{
		UserUuid: strings.TrimSpace(userUUID), DeviceID: strings.TrimSpace(deviceID), GroupUuid: strings.TrimSpace(groupUUID), PulledMessageSeq: messageSeq,
	})
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
	locators := make([]model.SyncMessageLocator, 0, len(inbox))
	for _, row := range inbox {
		locators = append(locators, model.SyncMessageLocator{MessageUUID: row.MessageUuid, ConversationKey: row.ConversationKey, MessageSeq: row.MessageSeq})
	}
	byID, err := r.hydrator.Hydrate(ctx, locators)
	if err != nil {
		return nil, err
	}
	items := make([]*model.SyncMessage, 0, len(inbox))
	for _, row := range inbox {
		message := byID[row.MessageUuid]
		if message == nil {
			return nil, fmt.Errorf("sync inbox message %s is missing", row.MessageUuid)
		}
		items = append(items, &model.SyncMessage{SyncSeq: row.SyncSeq, ConversationKey: row.ConversationKey, MessageUUID: row.MessageUuid, MessageSeq: row.MessageSeq, Message: message})
	}
	return items, nil
}
