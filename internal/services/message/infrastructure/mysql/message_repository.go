package messagemysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/mapper"
)

var _ application.MessageStore = (*MessageRepository)(nil)
var _ application.MessageMetadataStore = (*MessageRepository)(nil)

type MessageRepository struct {
	store          mysqlData.TransactionStore
	writeSyncInbox bool
}

func NewMessageRepository(store mysqlData.TransactionStore) (*MessageRepository, error) {
	return NewMessageRepositoryWithInboxWrites(store, true)
}

func NewMessageRepositoryWithInboxWrites(store mysqlData.TransactionStore, enabled bool) (*MessageRepository, error) {
	if store == nil {
		return nil, errors.New("message transaction store is required")
	}
	return &MessageRepository{store: store, writeSyncInbox: enabled}, nil
}

func (r *MessageRepository) CreateWithSync(message *model.Message, recipients []string) error {
	return r.storeWithSync(message, nil, recipients)
}

func (r *MessageRepository) StoreWithOutboxAndSync(message *model.Message, buildOutbox application.MessageOutboxBuilder, recipients []string) error {
	return r.storeWithSync(message, buildOutbox, recipients)
}

func (r *MessageRepository) storeWithSync(message *model.Message, buildOutbox application.MessageOutboxBuilder, recipients []string) error {
	if message == nil {
		return errors.New("store message with sqlc: message is required")
	}
	originalMessage := *message
	ctx := context.Background()
	err := r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		sequence, err := allocateConversationSequence(ctx, q, message.ConversationKey)
		if err != nil {
			return err
		}
		message.Seq = sequence
		if _, err := q.CreateMessage(ctx, mapper.MessageCreateParams(message)); err != nil {
			return fmt.Errorf("create message with sqlc: %w", err)
		}
		row, err := q.GetMessageByUUID(ctx, message.UUID)
		if err != nil {
			return fmt.Errorf("reload message with sqlc: %w", err)
		}
		*message = *mapper.Message(row)
		if err := q.CreateMessageMetadata(ctx, mapper.MessageMetadataCreateParams(message)); err != nil {
			return fmt.Errorf("create message metadata with sqlc: %w", err)
		}
		if message.TargetType == model.MessageTargetGroup {
			if err := q.UpsertGroupSyncState(ctx, generated.UpsertGroupSyncStateParams{GroupUuid: message.TargetUUID, LatestMessageSeq: message.Seq, LatestMessageUuid: message.UUID}); err != nil {
				return fmt.Errorf("advance group sync state with sqlc: %w", err)
			}
		}
		if r.writeSyncInbox {
			if err := createSQLCSyncInbox(ctx, q, message, recipients); err != nil {
				return err
			}
		}
		if buildOutbox != nil {
			event, err := buildOutbox(message)
			if err != nil {
				return fmt.Errorf("build outbox event after sequence allocation: %w", err)
			}
			if event == nil {
				return errors.New("build outbox event after sequence allocation: event is required")
			}
			if event.Status == "" {
				event.Status = model.OutboxStatusPending
			}
			if _, err := q.CreateOutboxEvent(ctx, mapper.OutboxCreateParams(event)); err != nil {
				return fmt.Errorf("enqueue outbox event with sqlc: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		*message = originalMessage
	}
	return err
}

func allocateConversationSequence(ctx context.Context, q *generated.Queries, conversationKey string) (uint64, error) {
	conversationKey = strings.TrimSpace(conversationKey)
	if conversationKey == "" {
		return 0, errors.New("allocate conversation sequence: conversation key is required")
	}
	if _, err := q.EnsureConversationSequence(ctx, conversationKey); err != nil {
		return 0, fmt.Errorf("ensure conversation sequence with sqlc: %w", err)
	}
	lastSeq, err := q.LockConversationSequence(ctx, conversationKey)
	if err != nil {
		return 0, fmt.Errorf("lock conversation sequence with sqlc: %w", err)
	}
	if lastSeq == ^uint64(0) {
		return 0, fmt.Errorf("allocate conversation sequence: sequence exhausted for %s", conversationKey)
	}
	nextSeq := lastSeq + 1
	if err := q.AdvanceConversationSequence(ctx, generated.AdvanceConversationSequenceParams{LastSeq: nextSeq, ConversationKey: conversationKey}); err != nil {
		return 0, fmt.Errorf("advance conversation sequence with sqlc: %w", err)
	}
	return nextSeq, nil
}

func (r *MessageRepository) EnsureSyncInbox(message *model.Message, recipients []string) error {
	if !r.writeSyncInbox {
		return nil
	}
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		return createSQLCSyncInbox(ctx, q, message, recipients)
	})
}

func (r *MessageRepository) EnsureOutbox(event *model.OutboxEvent) error {
	if event == nil {
		return nil
	}
	if event.Status == "" {
		event.Status = model.OutboxStatusPending
	}
	_, err := r.store.Queries().CreateOutboxEvent(context.Background(), mapper.OutboxCreateParams(event))
	return err
}

func (r *MessageRepository) GetByUUID(uuid string) (*model.Message, error) {
	row, err := r.store.Queries().GetMessageByUUID(context.Background(), uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.Message(row), nil
}

func (r *MessageRepository) GetBySenderAndClientMessageID(senderUUID, clientID string) (*model.Message, error) {
	row, err := r.store.Queries().GetMessageBySenderAndClientID(context.Background(), generated.GetMessageBySenderAndClientIDParams{SenderUuid: senderUUID, ClientMessageID: clientID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.Message(row), nil
}

func (r *MessageRepository) GetMetadataByUUID(uuid string) (*model.MessageMetadata, error) {
	row, err := r.store.Queries().GetMessageMetadataByUUID(context.Background(), strings.TrimSpace(uuid))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.MessageMetadata(row), nil
}

func (r *MessageRepository) GetMetadataBySenderAndClientMessageID(senderUUID, clientID string) (*model.MessageMetadata, error) {
	row, err := r.store.Queries().GetMessageMetadataBySenderAndClientID(
		context.Background(),
		generated.GetMessageMetadataBySenderAndClientIDParams{
			SenderUuid:      strings.TrimSpace(senderUUID),
			ClientMessageID: strings.TrimSpace(clientID),
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.MessageMetadata(row), nil
}

func (r *MessageRepository) HasConversationMessages(key string) (bool, error) {
	return r.store.Queries().HasConversationMessages(context.Background(), key)
}

func (r *MessageRepository) ListByConversationKey(key string, beforeID uint, limit int) ([]*model.Message, error) {
	rows, err := r.store.Queries().ListMessagesByConversationBefore(context.Background(), generated.ListMessagesByConversationBeforeParams{ConversationKey: key, BeforeID: uint64(beforeID), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	messages := mapper.Messages(rows)
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *MessageRepository) ListByConversationKeyAfter(key string, afterID uint, limit int) ([]*model.Message, error) {
	rows, err := r.store.Queries().ListMessagesByConversationAfter(context.Background(), generated.ListMessagesByConversationAfterParams{ConversationKey: key, AfterID: uint64(afterID), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return mapper.Messages(rows), nil
}

func (r *MessageRepository) ListByConversationSeqBefore(key string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	rows, err := r.store.Queries().ListMessagesByConversationSeqBefore(context.Background(), generated.ListMessagesByConversationSeqBeforeParams{ConversationKey: key, BeforeSeq: beforeSeq, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	messages := mapper.Messages(rows)
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *MessageRepository) ListByConversationSeqAfter(key string, afterSeq uint64, limit int) ([]*model.Message, error) {
	rows, err := r.store.Queries().ListMessagesByConversationSeqAfter(context.Background(), generated.ListMessagesByConversationSeqAfterParams{ConversationKey: key, AfterSeq: afterSeq, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return mapper.Messages(rows), nil
}

func (r *MessageRepository) ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	userUUID = strings.TrimSpace(userUUID)
	rows, err := r.store.Queries().ListOfflineMessagesByUser(context.Background(), generated.ListOfflineMessagesByUserParams{AfterID: uint64(afterID), DirectType: model.MessageTargetDirect, UserUuid: userUUID, GroupType: model.MessageTargetGroup, GroupNormalStatus: model.GroupStatusNormal, GroupDismissedStatus: model.GroupStatusDismissed, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return mapper.Messages(rows), nil
}

func (r *MessageRepository) FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error) {
	row, err := r.store.Queries().FindLatestAccessibleFileMetadata(context.Background(), generated.FindLatestAccessibleFileMetadataParams{FileUuid: fileUUID, FileMessageType: model.MessageTypeFile, DirectType: model.MessageTargetDirect, UserUuid: userUUID, GroupType: model.MessageTargetGroup, GroupNormalStatus: model.GroupStatusNormal, GroupDismissedStatus: model.GroupStatusDismissed})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	metadata := mapper.MessageMetadata(row)
	return &model.Message{
		UUID: metadata.MessageUUID, Seq: metadata.MessageSeq,
		ConversationKey: metadata.ConversationKey, SenderUUID: metadata.SenderUUID,
		TargetType: metadata.TargetType, TargetUUID: metadata.TargetUUID,
		MessageType: metadata.MessageType, FileID: metadata.FileID,
		FileExpiresAt: metadata.FileExpiresAt, SentAt: metadata.SentAt,
	}, nil
}

func createSQLCSyncInbox(ctx context.Context, q *generated.Queries, message *model.Message, recipients []string) error {
	if message == nil {
		return nil
	}
	for _, userUUID := range uniqueSortedSQLCUUIDs(recipients) {
		if _, err := q.EnsureUserSyncState(ctx, userUUID); err != nil {
			return fmt.Errorf("ensure sync state with sqlc: %w", err)
		}
		if _, err := q.LockUserSyncState(ctx, userUUID); err != nil {
			return fmt.Errorf("lock sync state with sqlc: %w", err)
		}
		if _, err := q.CreateUserSyncInbox(ctx, generated.CreateUserSyncInboxParams{UserUuid: userUUID, MessageUuid: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq}); err != nil {
			return fmt.Errorf("create sync inbox with sqlc: %w", err)
		}
	}
	return nil
}
