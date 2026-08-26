package httpdto

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
)

type SyncMessageResponse struct {
	SyncSeq         uint64           `json:"sync_seq"`
	ConversationKey string           `json:"conversation_key"`
	Message         *MessageResponse `json:"message"`
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
			Message:         ToMessageResponse(item.Message),
		})
	}
	return &SyncPageResponse{Items: items, NextSeq: page.NextSeq, HasMore: page.HasMore}
}
