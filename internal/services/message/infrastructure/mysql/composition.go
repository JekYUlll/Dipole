package messagemysql

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

// ProcessRepositories contains only the persistence adapters owned by the
// Message process. It is safe to use from an independent Composition Root.
type ProcessRepositories struct {
	Messages             application.MessageStore
	Outbox               application.OutboxRelayStore
	ConversationSequence *ConversationSequenceRepository
}

func NewProcessRepositories(db *sql.DB, inboxWrites bool) (*ProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("message repository composition requires database/sql connection")
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create message transaction store: %w", err)
	}
	messages, err := NewMessageRepositoryWithInboxWrites(store, inboxWrites)
	if err != nil {
		return nil, fmt.Errorf("create message repository: %w", err)
	}
	outbox, err := NewOutboxRepository(store)
	if err != nil {
		return nil, fmt.Errorf("create message outbox repository: %w", err)
	}
	return &ProcessRepositories{
		Messages:             messages,
		Outbox:               outbox,
		ConversationSequence: NewConversationSequenceRepository(generated.New(db)),
	}, nil
}
