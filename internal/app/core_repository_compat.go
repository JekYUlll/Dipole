package app

import (
	"database/sql"

	"github.com/JekYUlll/Dipole/internal/application"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

// Core repository symbols remain available to embedded callers during the
// service extraction. New standalone code should use coremysql directly.
type CoreProcessRepositories = coremysql.ProcessRepositories

func NewCoreProcessRepositories(db *sql.DB) (*CoreProcessRepositories, error) {
	return coremysql.NewProcessRepositories(db)
}

type CachedUserStore = coremysql.CachedUserStore
type CachedGroupStore = coremysql.CachedGroupStore
type CachedContactStore = coremysql.CachedContactStore

func NewCachedUserStore(backend application.UserStore) *CachedUserStore {
	return coremysql.NewCachedUserStore(backend)
}

func NewCachedGroupStore(backend application.GroupStore) *CachedGroupStore {
	return coremysql.NewCachedGroupStore(backend)
}

func NewCachedContactStore(backend application.ContactStore) *CachedContactStore {
	return coremysql.NewCachedContactStore(backend)
}
