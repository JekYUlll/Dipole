package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

type Store struct {
	db      *sql.DB
	queries *generated.Queries
}

// TransactionStore is the minimal sqlc transaction boundary shared by
// service-owned repository adapters.
type TransactionStore interface {
	Queries() *generated.Queries
	WithinTx(context.Context, *sql.TxOptions, func(*generated.Queries) error) error
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("mysql database is required")
	}
	return &Store{db: db, queries: generated.New(db)}, nil
}

func (s *Store) Queries() *generated.Queries {
	return s.queries
}

func (s *Store) WithinTx(ctx context.Context, options *sql.TxOptions, fn func(*generated.Queries) error) error {
	if fn == nil {
		return errors.New("transaction callback is required")
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin mysql transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(generated.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mysql transaction: %w", err)
	}
	return nil
}

func IsDuplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
