package mysql

import (
	"errors"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestNewStoreRequiresDatabase(t *testing.T) {
	t.Parallel()

	if _, err := NewStore(nil); err == nil {
		t.Fatal("expected nil database to fail")
	}
}

func TestIsDuplicateKey(t *testing.T) {
	t.Parallel()

	if !IsDuplicateKey(&mysqlDriver.MySQLError{Number: 1062}) {
		t.Fatal("expected duplicate key error")
	}
	if IsDuplicateKey(errors.New("other")) {
		t.Fatal("did not expect generic error to be duplicate key")
	}
}
