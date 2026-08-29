package coremysql

import mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"

type transactionStore = mysqlData.TransactionStore

var _ transactionStore = (*mysqlData.Store)(nil)
