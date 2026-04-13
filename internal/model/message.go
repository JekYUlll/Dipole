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
)

type Message struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UUID            string    `gorm:"size:24;uniqueIndex;not null" json:"uuid"`
	ConversationKey string    `gorm:"size:64;index;not null" json:"-"`
	SenderUUID      string    `gorm:"column:sender_uuid;size:24;index;not null" json:"sender_uuid"`
	TargetType      int8      `gorm:"column:target_type;not null;default:0" json:"target_type"`
	TargetUUID      string    `gorm:"column:target_uuid;size:24;index;not null" json:"target_uuid"`
	MessageType     int8      `gorm:"column:message_type;not null;default:0" json:"message_type"`
	Content         string    `gorm:"type:text;not null" json:"content"`
	SentAt          time.Time `gorm:"not null;index" json:"sent_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Message) TableName() string {
	return "messages"
}

func DirectConversationKey(userOneUUID, userTwoUUID string) string {
	users := []string{
		strings.TrimSpace(userOneUUID),
		strings.TrimSpace(userTwoUUID),
	}
	sort.Strings(users)

	return "direct:" + users[0] + ":" + users[1]
}
