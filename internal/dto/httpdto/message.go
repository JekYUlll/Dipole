package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type MessageResponse struct {
	ID          uint      `json:"id"`
	MessageID   string    `json:"message_id"`
	FromUUID    string    `json:"from_uuid"`
	TargetUUID  string    `json:"target_uuid"`
	TargetType  int8      `json:"target_type"`
	MessageType int8      `json:"message_type"`
	Content     string    `json:"content"`
	FileID      string    `json:"file_id,omitempty"`
	FileName    string    `json:"file_name,omitempty"`
	FileSize    int64     `json:"file_size,omitempty"`
	FileURL     string    `json:"file_url,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	SentAt      time.Time `json:"sent_at"`
}

func ToMessageResponse(message *model.Message) *MessageResponse {
	if message == nil {
		return nil
	}

	return &MessageResponse{
		ID:          message.ID,
		MessageID:   message.UUID,
		FromUUID:    message.SenderUUID,
		TargetUUID:  message.TargetUUID,
		TargetType:  message.TargetType,
		MessageType: message.MessageType,
		Content:     message.Content,
		FileID:      message.FileID,
		FileName:    message.FileName,
		FileSize:    message.FileSize,
		FileURL:     message.FileURL,
		ContentType: message.FileContentType,
		SentAt:      message.SentAt,
	}
}

func ToMessageResponses(messages []*model.Message) []*MessageResponse {
	response := make([]*MessageResponse, 0, len(messages))
	for _, message := range messages {
		response = append(response, ToMessageResponse(message))
	}

	return response
}
