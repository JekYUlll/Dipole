package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type SearchMessageResponse struct {
	MessageID       string    `json:"message_id"`
	ConversationKey string    `json:"conversation_key"`
	MessageSeq      uint64    `json:"message_seq"`
	Revision        uint64    `json:"revision"`
	FromUUID        string    `json:"from_uuid"`
	MessageType     int8      `json:"message_type"`
	Content         string    `json:"content"`
	SentAt          time.Time `json:"sent_at"`
}

func ToSearchMessageResponses(documents []*model.MessageSearchDocument) []*SearchMessageResponse {
	responses := make([]*SearchMessageResponse, 0, len(documents))
	for _, document := range documents {
		if document == nil {
			continue
		}
		responses = append(responses, &SearchMessageResponse{
			MessageID: document.MessageUUID, ConversationKey: document.ConversationKey,
			MessageSeq: document.MessageSeq, Revision: document.Revision, FromUUID: document.SenderUUID,
			MessageType: document.MessageType, Content: document.Content, SentAt: document.SentAt,
		})
	}
	return responses
}
