package model

import (
	"sort"
	"strings"
	"time"
)

const (
	MessageTargetDirect int8 = iota
	MessageTargetGroup
)

const (
	MessageTypeText int8 = iota
	MessageTypeFile
	MessageTypeAIText
	MessageTypeSystem
)

type Message struct {
	ID              uint       `json:"id"`
	UUID            string     `json:"uuid"`
	ClientMessageID string     `json:"-"`
	ConversationKey string     `json:"-"`
	SenderUUID      string     `json:"sender_uuid"`
	TargetType      int8       `json:"target_type"`
	TargetUUID      string     `json:"target_uuid"`
	MessageType     int8       `json:"message_type"`
	Content         string     `json:"content"`
	FileID          string     `json:"file_id"`
	FileName        string     `json:"file_name"`
	FileSize        int64      `json:"file_size"`
	FileURL         string     `json:"file_url"`
	FileContentType string     `json:"file_content_type"`
	FileExpiresAt   *time.Time `json:"file_expires_at,omitempty"`
	SentAt          time.Time  `json:"sent_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func DirectConversationKey(userOneUUID, userTwoUUID string) string {
	users := []string{strings.TrimSpace(userOneUUID), strings.TrimSpace(userTwoUUID)}
	sort.Strings(users)
	return "direct:" + users[0] + ":" + users[1]
}

func GroupConversationKey(groupUUID string) string {
	return "group:" + strings.TrimSpace(groupUUID)
}
