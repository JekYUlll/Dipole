package mapper

import (
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func Conversation(row generated.Conversation) *model.Conversation {
	return &model.Conversation{
		ID:                    uint(row.ID),
		UserUUID:              row.UserUuid,
		TargetType:            row.TargetType,
		TargetUUID:            row.TargetUuid,
		ConversationKey:       row.ConversationKey,
		LastMessageUUID:       row.LastMessageUuid,
		LastMessageSeq:        row.LastMessageSeq,
		ReadSeq:               row.ReadSeq,
		LastMessageType:       row.LastMessageType,
		LastMessagePreview:    row.LastMessagePreview,
		LastMessageAt:         row.LastMessageAt,
		LastMessageSenderUUID: row.LastMessageSenderUuid,
		UnreadCount:           int(row.UnreadCount),
		Remark:                row.Remark,
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

func Conversations(rows []generated.Conversation) []*model.Conversation {
	conversations := make([]*model.Conversation, 0, len(rows))
	for _, row := range rows {
		conversations = append(conversations, Conversation(row))
	}
	return conversations
}
