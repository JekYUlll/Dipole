package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.MessageStore = (*MessageRepository)(nil)

type MessageRepository struct{ store transactionStore }

func NewMessageRepository(store transactionStore) (*MessageRepository, error) {
	if store == nil {
		return nil, errors.New("message transaction store is required")
	}
	return &MessageRepository{store: store}, nil
}

func (r *MessageRepository) CreateWithSync(message *model.Message, recipients []string) error {
	return r.storeWithSync(message, nil, recipients)
}

func (r *MessageRepository) StoreWithOutboxAndSync(message *model.Message, event *model.OutboxEvent, recipients []string) error {
	return r.storeWithSync(message, event, recipients)
}

func (r *MessageRepository) storeWithSync(message *model.Message, event *model.OutboxEvent, recipients []string) error {
	if message == nil {
		return errors.New("store message with sqlc: message is required")
	}
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		if _, err := q.CreateMessage(ctx, mapper.MessageCreateParams(message)); err != nil {
			return fmt.Errorf("create message with sqlc: %w", err)
		}
		row, err := q.GetMessageByUUID(ctx, message.UUID)
		if err != nil {
			return fmt.Errorf("reload message with sqlc: %w", err)
		}
		*message = *mapper.Message(row)
		if err := createSQLCSyncInbox(ctx, q, message, recipients); err != nil {
			return err
		}
		if event != nil {
			if event.Status == "" {
				event.Status = model.OutboxStatusPending
			}
			if _, err := q.CreateOutboxEvent(ctx, mapper.OutboxCreateParams(event)); err != nil {
				return fmt.Errorf("enqueue outbox event with sqlc: %w", err)
			}
		}
		return nil
	})
}

func (r *MessageRepository) EnsureSyncInbox(message *model.Message, recipients []string) error {
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

func (r *MessageRepository) ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	userUUID = strings.TrimSpace(userUUID)
	rows, err := r.store.Queries().ListOfflineMessagesByUser(context.Background(), generated.ListOfflineMessagesByUserParams{AfterID: uint64(afterID), DirectType: model.MessageTargetDirect, UserUuid: userUUID, GroupType: model.MessageTargetGroup, GroupNormalStatus: model.GroupStatusNormal, GroupDismissedStatus: model.GroupStatusDismissed, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return mapper.Messages(rows), nil
}

func (r *MessageRepository) FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error) {
	row, err := r.store.Queries().FindLatestAccessibleFileMessage(context.Background(), generated.FindLatestAccessibleFileMessageParams{FileUuid: fileUUID, FileMessageType: model.MessageTypeFile, DirectType: model.MessageTargetDirect, UserUuid: userUUID, GroupType: model.MessageTargetGroup, GroupNormalStatus: model.GroupStatusNormal, GroupDismissedStatus: model.GroupStatusDismissed})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.Message(row), nil
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
		if _, err := q.CreateUserSyncInbox(ctx, generated.CreateUserSyncInboxParams{UserUuid: userUUID, MessageUuid: message.UUID, ConversationKey: message.ConversationKey}); err != nil {
			return fmt.Errorf("create sync inbox with sqlc: %w", err)
		}
	}
	return nil
}

func uniqueSortedSQLCUUIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
