package syncmysql

import (
	"database/sql"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

// ProcessRepositories contains persistence adapters owned by the Sync
// process. Message body hydration remains an explicit application port.
type ProcessRepositories struct {
	Sync       application.SyncStore
	Projection application.SyncProjectionStore
}

func NewProcessRepositories(db *sql.DB, hydrator application.SyncMessageHydrator) (*ProcessRepositories, error) {
	if db == nil {
		return nil, fmt.Errorf("Sync repository composition requires database/sql connection")
	}
	queries := generated.New(db)
	var syncStore *SyncRepository
	var err error
	if hydrator == nil {
		syncStore, err = NewSyncRepository(queries)
	} else {
		syncStore, err = NewSyncRepositoryWithHydrator(queries, hydrator)
	}
	if err != nil {
		return nil, fmt.Errorf("create Sync repository: %w", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("create Sync transaction store: %w", err)
	}
	projection, err := NewSyncProjectionRepository(store)
	if err != nil {
		return nil, fmt.Errorf("create Sync projection repository: %w", err)
	}
	return &ProcessRepositories{Sync: syncStore, Projection: projection}, nil
}
