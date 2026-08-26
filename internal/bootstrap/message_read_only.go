package bootstrap

import (
	"errors"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

var errMessageShadowCommand = errors.New("message shadow runtime rejects commands")

type queryOnlyMessageApplication struct {
	queries application.MessageQuery
}

func newQueryOnlyMessageApplication(queries application.MessageQuery) *queryOnlyMessageApplication {
	return &queryOnlyMessageApplication{queries: queries}
}

func (*queryOnlyMessageApplication) SendDirectMessage(string, string, string, string) (*model.Message, error) {
	return nil, errMessageShadowCommand
}

func (*queryOnlyMessageApplication) SendGroupMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, errMessageShadowCommand
}

func (*queryOnlyMessageApplication) SendDirectFileMessage(string, string, string, string) (*model.Message, error) {
	return nil, errMessageShadowCommand
}

func (*queryOnlyMessageApplication) SendGroupFileMessage(string, string, string, string) (*model.Message, []string, error) {
	return nil, nil, errMessageShadowCommand
}

func (a *queryOnlyMessageApplication) ListDirectMessages(userUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return a.queries.ListDirectMessages(userUUID, targetUUID, beforeID, limit)
}

func (a *queryOnlyMessageApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	return a.queries.ListGroupMessages(userUUID, groupUUID, beforeID, limit)
}

func (a *queryOnlyMessageApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return a.queries.ListGroupMessagesAfter(userUUID, groupUUID, afterID, limit)
}

func (a *queryOnlyMessageApplication) ListOfflineMessages(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	return a.queries.ListOfflineMessages(userUUID, afterID, limit)
}
