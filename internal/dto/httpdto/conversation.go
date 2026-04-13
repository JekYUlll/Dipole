package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/service"
)

type ConversationMessageSummaryResponse struct {
	MessageID   string    `json:"message_id"`
	MessageType int8      `json:"message_type"`
	Preview     string    `json:"preview"`
	SentAt      time.Time `json:"sent_at"`
}

type ConversationResponse struct {
	ConversationKey string                             `json:"conversation_key"`
	TargetType      int8                               `json:"target_type"`
	TargetUser      *PublicUserResponse                `json:"target_user,omitempty"`
	LastMessage     ConversationMessageSummaryResponse `json:"last_message"`
	UnreadCount     int                                `json:"unread_count"`
}

func ToConversationResponse(item *service.ConversationView) *ConversationResponse {
	if item == nil || item.Conversation == nil {
		return nil
	}

	return &ConversationResponse{
		ConversationKey: item.Conversation.ConversationKey,
		TargetType:      item.Conversation.TargetType,
		TargetUser:      ToPublicUserResponse(item.TargetUser),
		LastMessage: ConversationMessageSummaryResponse{
			MessageID:   item.Conversation.LastMessageUUID,
			MessageType: item.Conversation.LastMessageType,
			Preview:     item.Conversation.LastMessagePreview,
			SentAt:      item.Conversation.LastMessageAt,
		},
		UnreadCount: item.Conversation.UnreadCount,
	}
}

func ToConversationResponses(items []*service.ConversationView) []*ConversationResponse {
	response := make([]*ConversationResponse, 0, len(items))
	for _, item := range items {
		response = append(response, ToConversationResponse(item))
	}

	return response
}
