package httpdto

import (
	"time"

	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
)

type ConversationMessageSummaryResponse struct {
	MessageID   string    `json:"message_id"`
	MessageType int8      `json:"message_type"`
	Preview     string    `json:"preview"`
	SentAt      time.Time `json:"sent_at"`
	SenderUUID  string    `json:"sender_uuid"`
}

type ConversationResponse struct {
	ConversationKey string                             `json:"conversation_key"`
	TargetType      int8                               `json:"target_type"`
	TargetUser      *PublicUserResponse                `json:"target_user,omitempty"`
	Remark          string                             `json:"remark"`
	LastMessage     ConversationMessageSummaryResponse `json:"last_message"`
	UnreadCount     int                                `json:"unread_count"`
	LastMessageSeq  uint64                             `json:"last_message_seq"`
	ReadSeq         uint64                             `json:"read_seq"`
}

type UpdateConversationRemarkRequest struct {
	Remark string `json:"remark" binding:"max=50"`
}

func ToConversationResponse(item *coreconversation.ConversationView) *ConversationResponse {
	if item == nil || item.Conversation == nil {
		return nil
	}

	return &ConversationResponse{
		ConversationKey: item.Conversation.ConversationKey,
		TargetType:      item.Conversation.TargetType,
		TargetUser:      ToPublicUserResponse(item.TargetUser),
		Remark:          item.Conversation.Remark,
		LastMessage: ConversationMessageSummaryResponse{
			MessageID:   item.Conversation.LastMessageUUID,
			MessageType: item.Conversation.LastMessageType,
			Preview:     item.Conversation.LastMessagePreview,
			SentAt:      item.Conversation.LastMessageAt,
			SenderUUID:  item.Conversation.LastMessageSenderUUID,
		},
		UnreadCount:    item.Conversation.UnreadCount,
		LastMessageSeq: item.Conversation.LastMessageSeq,
		ReadSeq:        item.Conversation.ReadSeq,
	}
}

func ToConversationResponses(items []*coreconversation.ConversationView) []*ConversationResponse {
	response := make([]*ConversationResponse, 0, len(items))
	for _, item := range items {
		response = append(response, ToConversationResponse(item))
	}

	return response
}
