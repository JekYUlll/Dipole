package repository

import (
	"github.com/JekYUlll/Dipole/internal/application"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
)

type SyncRepository = syncmysql.SyncRepository
type SyncProjectionRepository = syncmysql.SyncProjectionRepository
type MySQLSyncMessageHydrator = syncmysql.MySQLSyncMessageHydrator

func NewSyncRepository(queries *generated.Queries) (*SyncRepository, error) {
	return syncmysql.NewSyncRepository(queries)
}

func NewSyncRepositoryWithHydrator(queries *generated.Queries, hydrator application.SyncMessageHydrator) (*SyncRepository, error) {
	return syncmysql.NewSyncRepositoryWithHydrator(queries, hydrator)
}

func NewMySQLSyncMessageHydrator(queries *generated.Queries) (*MySQLSyncMessageHydrator, error) {
	return syncmysql.NewMySQLSyncMessageHydrator(queries)
}

func NewSyncProjectionRepository(store *mysqlData.Store) (*SyncProjectionRepository, error) {
	return syncmysql.NewSyncProjectionRepository(store)
}
