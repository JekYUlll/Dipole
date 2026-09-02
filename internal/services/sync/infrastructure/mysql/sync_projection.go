package syncmysql

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqldata "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type SyncProjectionRepository struct{ store *mysqldata.Store }

var _ application.SyncProjectionStore = (*SyncProjectionRepository)(nil)

func NewSyncProjectionRepository(store *mysqldata.Store) (*SyncProjectionRepository, error) {
	if store == nil {
		return nil, errors.New("Sync projection transaction store is required")
	}
	return &SyncProjectionRepository{store: store}, nil
}

func (r *SyncProjectionRepository) Apply(projection *model.SyncProjection) error {
	if projection == nil {
		return errors.New("Sync projection is required")
	}
	messageUUID := strings.TrimSpace(projection.MessageUUID)
	conversationKey := strings.TrimSpace(projection.ConversationKey)
	if messageUUID == "" || conversationKey == "" || projection.MessageSeq == 0 {
		return errors.New("Sync projection locator is incomplete")
	}
	recipients := normalizedProjectionRecipients(projection.RecipientUUIDs)
	if len(recipients) == 0 {
		return errors.New("Sync projection recipients are required")
	}
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		for _, userUUID := range recipients {
			if _, err := queries.EnsureUserSyncState(ctx, userUUID); err != nil {
				return fmt.Errorf("ensure Sync projection state for %s: %w", userUUID, err)
			}
			if _, err := queries.LockUserSyncState(ctx, userUUID); err != nil {
				return fmt.Errorf("lock Sync projection state for %s: %w", userUUID, err)
			}
			if _, err := queries.CreateUserSyncInbox(ctx, generated.CreateUserSyncInboxParams{
				UserUuid: userUUID, MessageUuid: messageUUID, ConversationKey: conversationKey, MessageSeq: projection.MessageSeq,
			}); err != nil {
				return fmt.Errorf("append Sync projection for %s: %w", userUUID, err)
			}
		}
		return nil
	})
}

func normalizedProjectionRecipients(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
