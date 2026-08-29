package mysql

import platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"

// Store remains available to migration tools while the shared SQLC transaction
// boundary moves to the platform MySQL package.
type Store = platformmysql.Store
type TransactionStore = platformmysql.TransactionStore

var NewStore = platformmysql.NewStore
var IsDuplicateKey = platformmysql.IsDuplicateKey
