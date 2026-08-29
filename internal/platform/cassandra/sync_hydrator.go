package cassandra

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type TimelineLookup interface {
	Lookup(ctx context.Context, conversationKey string, sequence uint64) (TimelineRecord, bool, error)
}

type SyncMessageHydrator struct{ timeline TimelineLookup }

var _ application.SyncMessageHydrator = (*SyncMessageHydrator)(nil)

func NewSyncMessageHydrator(timeline TimelineLookup) (*SyncMessageHydrator, error) {
	if timeline == nil {
		return nil, fmt.Errorf("Cassandra Sync timeline is required")
	}
	return &SyncMessageHydrator{timeline: timeline}, nil
}

func (h *SyncMessageHydrator) Hydrate(ctx context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	result := make(map[string]*model.Message, len(locators))
	for _, locator := range locators {
		record, found, err := h.timeline.Lookup(ctx, locator.ConversationKey, locator.MessageSeq)
		if err != nil {
			return nil, fmt.Errorf("hydrate Sync message %s from Cassandra: %w", locator.MessageUUID, err)
		}
		if !found {
			return nil, fmt.Errorf("sync inbox message %s is missing", locator.MessageUUID)
		}
		projection := record.Projection
		if projection.MessageUUID != locator.MessageUUID {
			return nil, fmt.Errorf("sync inbox locator conflicts with message %s", locator.MessageUUID)
		}
		result[locator.MessageUUID] = messageFromTimelineProjection(projection)
	}
	return result, nil
}

func messageFromTimelineProjection(projection TimelineProjection) *model.Message {
	return &model.Message{
		UUID: projection.MessageUUID, ClientMessageID: projection.ClientMessageID,
		ConversationKey: projection.ConversationKey, Seq: projection.MessageSeq,
		SenderUUID: projection.SenderUUID, TargetType: projection.TargetType,
		TargetUUID: projection.TargetUUID, MessageType: projection.MessageType,
		Content: projection.Content, FileID: projection.FileID, FileName: projection.FileName,
		FileSize: projection.FileSize, FileURL: projection.FileURL,
		FileContentType: projection.FileContentType, FileExpiresAt: projection.FileExpiresAt,
		SentAt: projection.SentAt,
	}
}
