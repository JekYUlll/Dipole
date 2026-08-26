package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// MessageMetadata retains routing, idempotency, and file authorization facts
// independently from the full message body storage.
type MessageMetadata struct {
	MessageUUID     string
	ClientMessageID string
	ConversationKey string
	MessageSeq      uint64
	SenderUUID      string
	TargetType      int8
	TargetUUID      string
	MessageType     int8
	FileID          string
	FileExpiresAt   *time.Time
	PayloadSHA256   string
	SentAt          time.Time
}

func MetadataFromMessage(message *Message) *MessageMetadata {
	if message == nil {
		return nil
	}
	return &MessageMetadata{
		MessageUUID: message.UUID, ClientMessageID: message.ClientMessageID,
		ConversationKey: message.ConversationKey, MessageSeq: message.Seq,
		SenderUUID: message.SenderUUID, TargetType: message.TargetType,
		TargetUUID: message.TargetUUID, MessageType: message.MessageType,
		FileID: message.FileID, FileExpiresAt: message.FileExpiresAt,
		PayloadSHA256: MessagePayloadSHA256(message), SentAt: message.SentAt,
	}
}

func MessagePayloadSHA256(message *Message) string {
	if message == nil {
		return ""
	}
	var payload bytes.Buffer
	payload.WriteString("v1")
	payload.WriteByte(byte(message.MessageType))
	writeMetadataHashString(&payload, message.Content)
	writeMetadataHashString(&payload, message.FileID)
	sum := sha256.Sum256(payload.Bytes())
	return hex.EncodeToString(sum[:])
}

func writeMetadataHashString(payload *bytes.Buffer, value string) {
	_ = binary.Write(payload, binary.BigEndian, uint64(len(value)))
	payload.WriteString(value)
}
