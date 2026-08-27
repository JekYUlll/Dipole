package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

type MySQLSyncMessageHydrator struct{ queries *generated.Queries }

var _ application.SyncMessageHydrator = (*MySQLSyncMessageHydrator)(nil)

func NewMySQLSyncMessageHydrator(queries *generated.Queries) (*MySQLSyncMessageHydrator, error) {
	if queries == nil {
		return nil, fmt.Errorf("Sync message hydration queries are required")
	}
	return &MySQLSyncMessageHydrator{queries: queries}, nil
}

func (h *MySQLSyncMessageHydrator) Hydrate(ctx context.Context, locators []model.SyncMessageLocator) (map[string]*model.Message, error) {
	if len(locators) == 0 {
		return map[string]*model.Message{}, nil
	}
	ids := make([]string, 0, len(locators))
	for _, locator := range locators {
		ids = append(ids, strings.TrimSpace(locator.MessageUUID))
	}
	rows, err := h.queries.ListMessagesByUUIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*model.Message, len(rows))
	for _, row := range rows {
		message := mapper.Message(row)
		result[message.UUID] = message
	}
	if err := validateHydratedSyncMessages(locators, result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateHydratedSyncMessages(locators []model.SyncMessageLocator, messages map[string]*model.Message) error {
	for _, locator := range locators {
		message := messages[locator.MessageUUID]
		if message == nil {
			return fmt.Errorf("sync inbox message %s is missing", locator.MessageUUID)
		}
		if message.ConversationKey != locator.ConversationKey || message.Seq != locator.MessageSeq {
			return fmt.Errorf("sync inbox locator conflicts with message %s", locator.MessageUUID)
		}
	}
	return nil
}
