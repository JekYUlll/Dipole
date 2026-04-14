package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type MessageResponse struct {
	ID            uint       `json:"id"`
	MessageID     string     `json:"message_id"`
	FromUUID      string     `json:"from_uuid"`
	TargetUUID    string     `json:"target_uuid"`
	TargetType    int8       `json:"target_type"`
	MessageType   int8       `json:"message_type"`
	Content       string     `json:"content"`
	FileID        string     `json:"file_id,omitempty"`
	FileName      string     `json:"file_name,omitempty"`
	FileSize      int64      `json:"file_size,omitempty"`
	DownloadPath  string     `json:"download_path,omitempty"`
	ContentType   string     `json:"content_type,omitempty"`
	FileExpiresAt *time.Time `json:"file_expires_at,omitempty"`
	SentAt        time.Time  `json:"sent_at"`
}

func ToMessageResponse(message *model.Message) *MessageResponse {
	if message == nil {
		return nil
	}

	return &MessageResponse{
		ID:            message.ID,
		MessageID:     message.UUID,
		FromUUID:      message.SenderUUID,
		TargetUUID:    message.TargetUUID,
		TargetType:    message.TargetType,
		MessageType:   message.MessageType,
		Content:       message.Content,
		FileID:        message.FileID,
		FileName:      message.FileName,
		FileSize:      message.FileSize,
		DownloadPath:  fileDownloadPathForMessage(message),
		ContentType:   message.FileContentType,
		FileExpiresAt: message.FileExpiresAt,
		SentAt:        message.SentAt,
	}
}

func ToMessageResponses(messages []*model.Message) []*MessageResponse {
	response := make([]*MessageResponse, 0, len(messages))
	for _, message := range messages {
		response = append(response, ToMessageResponse(message))
	}

	return response
}

func fileDownloadPathForMessage(message *model.Message) string {
	if message == nil || message.MessageType != model.MessageTypeFile || message.FileID == "" {
		return ""
	}

	return FileDownloadPath(message.FileID)
}
