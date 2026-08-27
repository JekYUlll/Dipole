package httpdto

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type SyncMessageResponse struct {
	SyncSeq         uint64           `json:"sync_seq"`
	ConversationKey string           `json:"conversation_key"`
	MessageUUID     string           `json:"message_uuid"`
	MessageSeq      uint64           `json:"message_seq"`
	Message         *MessageResponse `json:"message"`
}

type AdvanceSyncCheckpointRequest struct {
	SyncSeq uint64 `json:"sync_seq"`
}

type ClientSyncComparisonRequest struct {
	Baseline         bool `json:"baseline"`
	Match            int  `json:"match"`
	Pending          int  `json:"pending"`
	LegacyOnly       int  `json:"legacy_only"`
	SyncOnly         int  `json:"sync_only"`
	Overflow         int  `json:"overflow"`
	StorageFull      int  `json:"storage_full"`
	SyncError        int  `json:"sync_error"`
	TimelineMatch    int  `json:"timeline_match"`
	TimelineMissing  int  `json:"timeline_missing"`
	TimelineMismatch int  `json:"timeline_mismatch"`
	TimelineError    int  `json:"timeline_error"`
	TimelineInvalid  int  `json:"timeline_invalid"`
}

type DeviceSyncCheckpointResponse struct {
	DeviceID string `json:"device_id"`
	SyncSeq  uint64 `json:"sync_seq"`
}

type AdvanceGroupSyncCheckpointRequest struct {
	MessageSeq uint64 `json:"message_seq"`
}

type GroupSyncCheckpointResponse struct {
	GroupUUID         string `json:"group_uuid"`
	LatestMessageSeq  uint64 `json:"latest_message_seq"`
	LatestMessageUUID string `json:"latest_message_id"`
	PulledMessageSeq  uint64 `json:"pulled_message_seq"`
}

func ToGroupSyncCheckpointResponse(checkpoint *model.GroupSyncCheckpoint) *GroupSyncCheckpointResponse {
	if checkpoint == nil {
		return &GroupSyncCheckpointResponse{}
	}
	return &GroupSyncCheckpointResponse{GroupUUID: checkpoint.GroupUUID, LatestMessageSeq: checkpoint.LatestMessageSeq, LatestMessageUUID: checkpoint.LatestMessageUUID, PulledMessageSeq: checkpoint.PulledMessageSeq}
}

func ToGroupSyncCheckpointResponses(checkpoints []*model.GroupSyncCheckpoint) []*GroupSyncCheckpointResponse {
	result := make([]*GroupSyncCheckpointResponse, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		result = append(result, ToGroupSyncCheckpointResponse(checkpoint))
	}
	return result
}

func ToDeviceSyncCheckpointResponse(checkpoint *model.DeviceSyncCheckpoint) *DeviceSyncCheckpointResponse {
	if checkpoint == nil {
		return &DeviceSyncCheckpointResponse{}
	}
	return &DeviceSyncCheckpointResponse{DeviceID: checkpoint.DeviceID, SyncSeq: checkpoint.SyncSeq}
}

type SyncPageResponse struct {
	Items   []*SyncMessageResponse `json:"items"`
	NextSeq uint64                 `json:"next_seq"`
	HasMore bool                   `json:"has_more"`
}

func ToSyncPageResponse(page *applicationPort.SyncPage) *SyncPageResponse {
	if page == nil {
		return &SyncPageResponse{}
	}
	items := make([]*SyncMessageResponse, 0, len(page.Items))
	for _, item := range page.Items {
		if item == nil {
			continue
		}
		items = append(items, &SyncMessageResponse{
			SyncSeq:         item.SyncSeq,
			ConversationKey: item.ConversationKey,
			MessageUUID:     item.MessageUUID,
			MessageSeq:      item.MessageSeq,
			Message:         ToMessageResponse(item.Message),
		})
	}
	return &SyncPageResponse{Items: items, NextSeq: page.NextSeq, HasMore: page.HasMore}
}
