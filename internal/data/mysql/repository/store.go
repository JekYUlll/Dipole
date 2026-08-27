package repository

import (
	"context"
	"database/sql"

	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type transactionStore interface {
	Queries() *generated.Queries
	WithinTx(context.Context, *sql.TxOptions, func(*generated.Queries) error) error
}

var _ transactionStore = (*mysqlData.Store)(nil)
