package kafka

import (
	"context"
	"sync"

	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type GroupHeatReader interface {
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

// NewGroupMessageHandler builds the Gateway group-message delivery handler.
// Hot groups receive a coalesced notify and fetch message bodies through sync.
func NewGroupMessageHandler(
	hub EventSender,
	hotGroups GroupHeatReader,
	notifier *Notifier,
	timelineNotifyMode string,
) platformKafka.Handler {
	return func(ctx context.Context, event platformKafka.Event) error {
		payload, err := decodeMessageEventPayload(event)
		if err != nil {
			logger.Warn("decode group message for delivery failed", zap.Error(err))
			return err
		}

		hot := false
		recentMessageCount := 0
		if hotGroups != nil {
			status, statusErr := hotGroups.Status(payload.TargetUUID, len(payload.RecipientUUIDs))
			if statusErr != nil {
				logger.Warn("query hot group status failed", zap.String("group_uuid", payload.TargetUUID), zap.Error(statusErr))
			} else {
				hot = status.IsHot
				recentMessageCount = status.RecentMessageCount
			}
		}

		eventData := chatMessageData(payload)
		var wg sync.WaitGroup
		for _, recipientUUID := range payload.RecipientUUIDs {
			if hot || recipientUUID == payload.SenderUUID {
				continue
			}
			wg.Add(1)
			go func(uuid string) {
				defer wg.Done()
				if timelineNotifyMode != wsTransport.TimelineNotifyPrimary {
					sendEventToUser(ctx, hub, uuid, wsTransport.TypeChatMessage, eventData)
				}
				if notify, ok := timelineNotifyData(event.Envelope, payload, timelineNotifyMode); ok {
					sendEventToUser(ctx, hub, uuid, wsTransport.TypeSyncItemNotifyV1, notify)
				}
			}(recipientUUID)
		}
		wg.Wait()
		if hot && notifier != nil {
			notifier.Enqueue(payload.TargetUUID, wsTransport.GroupMessageNotifyData{
				GroupUUID: payload.TargetUUID, LatestMessageID: payload.MessageID,
				LatestMessageSeq: payload.MessageSeq, MessageType: payload.MessageType,
				Preview: messagePreview(payload), RecentMessageCount: recentMessageCount,
				SentAt: payload.SentAt, SenderUUID: payload.SenderUUID,
			}, payload.RecipientUUIDs)
		}
		return nil
	}
}

func messagePreview(payload messagedomain.MessageEventPayload) string {
	if payload.MessageType == model.MessageTypeFile && payload.FileName != "" {
		return "[文件] " + payload.FileName
	}
	if payload.MessageType == model.MessageTypeFile {
		return "[文件]"
	}
	return payload.Content
}
