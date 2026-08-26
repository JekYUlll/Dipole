package model

import "time"

type MessageSearchDocument struct {
	MessageUUID     string
	ConversationKey string
	MessageSeq      uint64
	SenderUUID      string
	MessageType     int8
	Content         string
	SentAt          time.Time
}

type MessageSearchQuery struct {
	ConversationKeys []string
	Text             string
	Limit            int
}
