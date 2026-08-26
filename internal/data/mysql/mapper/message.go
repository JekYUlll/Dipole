package mapper

import (
	"database/sql"
	"time"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func MessageCreateParams(message *model.Message) generated.CreateMessageParams {
	return generated.CreateMessageParams{
		Uuid: message.UUID, ClientMessageID: message.ClientMessageID,
		ConversationKey: message.ConversationKey, Seq: message.Seq, SenderUuid: message.SenderUUID,
		TargetType: message.TargetType, TargetUuid: message.TargetUUID,
		MessageType: message.MessageType, Content: message.Content,
		FileID: message.FileID, FileName: message.FileName, FileSize: message.FileSize,
		FileUrl: message.FileURL, FileContentType: message.FileContentType,
		FileExpiresAt: nullableTime(message.FileExpiresAt), SentAt: message.SentAt,
	}
}

func MessageMetadataCreateParams(message *model.Message) generated.CreateMessageMetadataParams {
	return generated.CreateMessageMetadataParams{
		MessageUuid: message.UUID, ClientMessageID: message.ClientMessageID,
		ConversationKey: message.ConversationKey, MessageSeq: message.Seq,
		SenderUuid: message.SenderUUID, TargetType: message.TargetType,
		TargetUuid: message.TargetUUID, MessageType: message.MessageType,
		FileID: message.FileID, FileExpiresAt: nullableTime(message.FileExpiresAt),
		PayloadSha256: model.MessagePayloadSHA256(message), SentAt: message.SentAt,
	}
}

func MessageMetadata(row generated.MessageMetadatum) *model.MessageMetadata {
	return &model.MessageMetadata{
		MessageUUID: row.MessageUuid, ClientMessageID: row.ClientMessageID,
		ConversationKey: row.ConversationKey, MessageSeq: row.MessageSeq,
		SenderUUID: row.SenderUuid, TargetType: row.TargetType,
		TargetUUID: row.TargetUuid, MessageType: row.MessageType,
		FileID: row.FileID, FileExpiresAt: nullableTimePointer(row.FileExpiresAt),
		PayloadSHA256: row.PayloadSha256, SentAt: row.SentAt,
	}
}

func Message(row generated.Message) *model.Message {
	return &model.Message{
		ID: uint(row.ID), UUID: row.Uuid, ClientMessageID: row.ClientMessageID, Seq: row.Seq,
		ConversationKey: row.ConversationKey, SenderUUID: row.SenderUuid,
		TargetType: row.TargetType, TargetUUID: row.TargetUuid,
		MessageType: row.MessageType, Content: row.Content,
		FileID: row.FileID, FileName: row.FileName, FileSize: row.FileSize,
		FileURL: row.FileUrl, FileContentType: row.FileContentType,
		FileExpiresAt: nullableTimePointer(row.FileExpiresAt), SentAt: row.SentAt,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

func Messages(rows []generated.Message) []*model.Message {
	messages := make([]*model.Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, Message(row))
	}
	return messages
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
