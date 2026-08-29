package app

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
)

// Sync repository symbols remain available to embedded callers during the
// service extraction. New standalone code should use syncmysql directly.
type SyncProcessRepositories = syncmysql.ProcessRepositories

func NewSyncProcessRepositories(db *sql.DB) (*SyncProcessRepositories, error) {
	return syncmysql.NewProcessRepositories(db, nil)
}

func NewSyncProcessRepositoriesWithHydrator(db *sql.DB, hydrator application.SyncMessageHydrator) (*SyncProcessRepositories, error) {
	return syncmysql.NewProcessRepositories(db, hydrator)
}
