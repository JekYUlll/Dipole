package coremysql

import mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"

type transactionStore = mysqlData.TransactionStore

var _ transactionStore = (*mysqlData.Store)(nil)
