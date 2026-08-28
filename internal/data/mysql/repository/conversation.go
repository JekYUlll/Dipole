package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.ConversationStore = (*ConversationRepository)(nil)

type ConversationRepository struct {
	queries generated.Querier
}

func NewConversationRepository(queries generated.Querier) (*ConversationRepository, error) {
	if queries == nil {
		return nil, errors.New("conversation queries are required")
	}
	return &ConversationRepository{queries: queries}, nil
}

func (r *ConversationRepository) UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error {
	return r.upsertMessage(userUUID, model.MessageTargetDirect, targetUUID, message, unreadIncrement, "direct")
}

func (r *ConversationRepository) UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error {
	return r.upsertMessage(userUUID, model.MessageTargetGroup, groupUUID, message, unreadIncrement, "group")
}

func (r *ConversationRepository) UpsertGroupMessageBatch(groupUUID string, message *model.Message) error {
	if message == nil {
		return errors.New("upsert group conversation batch with sqlc: message is required")
	}
	initialReadSeq := uint64(0)
	if message.Seq > 0 {
		initialReadSeq = message.Seq - 1
	}
	_, err := r.queries.UpsertGroupConversationMessageBatch(context.Background(), generated.UpsertGroupConversationMessageBatchParams{
		TargetType: model.MessageTargetGroup, GroupUuid: groupUUID, ConversationKey: message.ConversationKey,
		LastMessageUuid: message.UUID, LastMessageSeq: message.Seq, SenderUuid: message.SenderUUID,
		InitialReadSeq: initialReadSeq, LastMessageType: message.MessageType,
		LastMessagePreview: model.BuildMessagePreview(message), LastMessageAt: message.SentAt,
		LastMessageSenderUuid: message.SenderUUID,
	})
	if err != nil {
		return fmt.Errorf("upsert group conversation batch with sqlc: %w", err)
	}
	return nil
}

func (r *ConversationRepository) upsertMessage(userUUID string, targetType int8, targetUUID string, message *model.Message, unreadIncrement int, kind string) error {
	if message == nil {
		return fmt.Errorf("upsert %s conversation with sqlc: message is required", kind)
	}
	initialReadSeq := message.Seq
	if unreadIncrement > 0 {
		increment := uint64(unreadIncrement)
		if message.Seq > increment {
			initialReadSeq = message.Seq - increment
		} else {
			initialReadSeq = 0
		}
	}
	_, err := r.queries.UpsertConversationMessage(context.Background(), generated.UpsertConversationMessageParams{
		UserUuid:              userUUID,
		TargetType:            targetType,
		TargetUuid:            targetUUID,
		ConversationKey:       message.ConversationKey,
		LastMessageUuid:       message.UUID,
		LastMessageSeq:        message.Seq,
		InitialReadSeq:        initialReadSeq,
		LastMessageType:       message.MessageType,
		LastMessagePreview:    model.BuildMessagePreview(message),
		LastMessageAt:         message.SentAt,
		LastMessageSenderUuid: message.SenderUUID,
		UnreadIncrement:       int64(unreadIncrement),
	})
	if err != nil {
		return fmt.Errorf("upsert %s conversation with sqlc: %w", kind, err)
	}
	return nil
}

func (r *ConversationRepository) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	rows, err := r.queries.ListConversationsByUser(context.Background(), generated.ListConversationsByUserParams{
		UserUuid: userUUID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list conversations by user UUID with sqlc: %w", err)
	}
	return mapper.Conversations(rows), nil
}

func (r *ConversationRepository) ListSearchConversationKeys(userUUID string) ([]string, error) {
	keys, err := r.queries.ListSearchConversationKeysByUser(context.Background(), generated.ListSearchConversationKeysByUserParams{
		UserUuid: userUUID, DirectTargetType: model.MessageTargetDirect,
		GroupNormalStatus: model.GroupStatusNormal, GroupDismissedStatus: model.GroupStatusDismissed,
	})
	if err != nil {
		return nil, fmt.Errorf("list Search conversation keys by user with sqlc: %w", err)
	}
	return keys, nil
}

func (r *ConversationRepository) GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error) {
	row, err := r.queries.GetConversationByUserAndKey(context.Background(), generated.GetConversationByUserAndKeyParams{
		UserUuid:        userUUID,
		ConversationKey: conversationKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation by user and key with sqlc: %w", err)
	}
	return mapper.Conversation(row), nil
}

func (r *ConversationRepository) InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error {
	_, err := r.queries.InitGroupConversation(context.Background(), generated.InitGroupConversationParams{
		UserUuid:        userUUID,
		TargetType:      model.MessageTargetGroup,
		TargetUuid:      groupUUID,
		ConversationKey: conversationKey,
		LastMessageAt:   createdAt,
	})
	if err != nil {
		return fmt.Errorf("init group conversation with sqlc: %w", err)
	}
	return nil
}

func (r *ConversationRepository) UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error {
	_, err := r.queries.UpdateConversationRemark(context.Background(), generated.UpdateConversationRemarkParams{
		Remark:          remark,
		UserUuid:        userUUID,
		ConversationKey: conversationKey,
	})
	if err != nil {
		return fmt.Errorf("update conversation remark with sqlc: %w", err)
	}
	return nil
}

func (r *ConversationRepository) MarkReadThroughByConversationKey(userUUID, conversationKey string, readThroughSeq uint64) error {
	if readThroughSeq > uint64(^uint64(0)>>1) {
		return errors.New("mark conversation read with sqlc: sequence exceeds signed query range")
	}
	_, err := r.queries.MarkConversationReadThrough(context.Background(), generated.MarkConversationReadThroughParams{
		ReadThroughSeq:  int64(readThroughSeq),
		UserUuid:        userUUID,
		ConversationKey: conversationKey,
	})
	if err != nil {
		return fmt.Errorf("mark conversation read through with sqlc: %w", err)
	}
	return nil
}
