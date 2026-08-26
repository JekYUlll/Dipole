package grpcmapping

import (
	"github.com/JekYUlll/Dipole/internal/model"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MessageToProto(message *model.Message) *messagev1.Message {
	if message == nil {
		return nil
	}
	result := &messagev1.Message{
		Id:              uint64(message.ID),
		ServerMessageId: message.UUID,
		ClientMessageId: message.ClientMessageID,
		ConversationKey: message.ConversationKey,
		Sequence:        message.Seq,
		SenderId:        message.SenderUUID,
		TargetType:      int32(message.TargetType),
		TargetId:        message.TargetUUID,
		MessageType:     int32(message.MessageType),
		Content:         message.Content,
		FileId:          message.FileID,
		FileName:        message.FileName,
		FileSize:        message.FileSize,
		FileUrl:         message.FileURL,
		FileContentType: message.FileContentType,
		SentAt:          timestamppb.New(message.SentAt),
	}
	if message.FileExpiresAt != nil {
		result.FileExpiresAt = timestamppb.New(*message.FileExpiresAt)
	}
	return result
}

func MessageFromProto(message *messagev1.Message) *model.Message {
	if message == nil {
		return nil
	}
	result := &model.Message{
		ID:              uint(message.GetId()),
		UUID:            message.GetServerMessageId(),
		ClientMessageID: message.GetClientMessageId(),
		ConversationKey: message.GetConversationKey(),
		Seq:             message.GetSequence(),
		SenderUUID:      message.GetSenderId(),
		TargetType:      int8(message.GetTargetType()),
		TargetUUID:      message.GetTargetId(),
		MessageType:     int8(message.GetMessageType()),
		Content:         message.GetContent(),
		FileID:          message.GetFileId(),
		FileName:        message.GetFileName(),
		FileSize:        message.GetFileSize(),
		FileURL:         message.GetFileUrl(),
		FileContentType: message.GetFileContentType(),
	}
	if message.GetSentAt() != nil {
		result.SentAt = message.GetSentAt().AsTime()
	}
	if message.GetFileExpiresAt() != nil {
		expiresAt := message.GetFileExpiresAt().AsTime()
		result.FileExpiresAt = &expiresAt
	}
	return result
}
