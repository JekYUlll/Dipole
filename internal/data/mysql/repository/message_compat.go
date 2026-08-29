package repository

import (
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
)

// MessageRepository remains available from the historical repository package
// while Message owns its MySQL adapter under the service boundary.
type MessageRepository = messagemysql.MessageRepository
type OutboxRepository = messagemysql.OutboxRepository
type ConversationSequenceRepository = messagemysql.ConversationSequenceRepository

func NewMessageRepository(store mysqlData.TransactionStore) (*MessageRepository, error) {
	return messagemysql.NewMessageRepository(store)
}

func NewMessageRepositoryWithInboxWrites(store mysqlData.TransactionStore, enabled bool) (*MessageRepository, error) {
	return messagemysql.NewMessageRepositoryWithInboxWrites(store, enabled)
}

func NewOutboxRepository(store mysqlData.TransactionStore) (*OutboxRepository, error) {
	return messagemysql.NewOutboxRepository(store)
}

func NewConversationSequenceRepository(queries *generated.Queries) *ConversationSequenceRepository {
	return messagemysql.NewConversationSequenceRepository(queries)
}
