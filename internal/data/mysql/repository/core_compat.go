package repository

import (
	"github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	coremysql "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

// Core repository aliases preserve embedded and maintenance callers while the
// implementations live under the Core service boundary.
type AdminRepository = coremysql.AdminRepository
type ContactRepository = coremysql.ContactRepository
type ConversationRepository = coremysql.ConversationRepository
type FileRepository = coremysql.FileRepository
type GroupRepository = coremysql.GroupRepository
type UserRepository = coremysql.UserRepository

func NewAdminRepository(queries generated.Querier) (*AdminRepository, error) {
	return coremysql.NewAdminRepository(queries)
}

func NewContactRepository(queries generated.Querier) (*ContactRepository, error) {
	return coremysql.NewContactRepository(queries)
}

func NewConversationRepository(queries generated.Querier) (*ConversationRepository, error) {
	return coremysql.NewConversationRepository(queries)
}

func NewFileRepository(queries generated.Querier) (*FileRepository, error) {
	return coremysql.NewFileRepository(queries)
}

func NewGroupRepository(store mysql.TransactionStore) (*GroupRepository, error) {
	return coremysql.NewGroupRepository(store)
}

func NewUserRepository(queries generated.Querier) (*UserRepository, error) {
	return coremysql.NewUserRepository(queries)
}
