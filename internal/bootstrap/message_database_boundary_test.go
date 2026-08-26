package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type databasePermissionProbeStub struct {
	allowedError error
	coreAllowed  string
	queries      []string
}

func (s *databasePermissionProbeStub) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	s.queries = append(s.queries, query)
	for _, table := range coreOwnedTables {
		if strings.Contains(query, "`"+table+"`") {
			if table == s.coreAllowed {
				return nil, nil
			}
			return nil, &mysqlDriver.MySQLError{Number: 1142, Message: "SELECT denied"}
		}
	}
	return nil, s.allowedError
}

func TestVerifyMessageDatabaseBoundaryAcceptsLeastPrivilegeAccount(t *testing.T) {
	probe := &databasePermissionProbeStub{}
	if err := verifyMessageDatabaseBoundary(context.Background(), probe); err != nil {
		t.Fatalf("verify message database boundary: %v", err)
	}
	wantQueries := len(messageOwnedTables) + len(coreOwnedTables)
	if len(probe.queries) != wantQueries {
		t.Fatalf("permission probes = %d, want %d", len(probe.queries), wantQueries)
	}
}

func TestVerifyMessageDatabaseBoundaryRejectsCoreTableAccess(t *testing.T) {
	err := verifyMessageDatabaseBoundary(context.Background(), &databasePermissionProbeStub{coreAllowed: "users"})
	if err == nil || !strings.Contains(err.Error(), "core-owned table users") {
		t.Fatalf("expected users access rejection, got %v", err)
	}
}

func TestVerifyMessageDatabaseBoundaryRejectsMissingOwnedTableAccess(t *testing.T) {
	err := verifyMessageDatabaseBoundary(context.Background(), &databasePermissionProbeStub{allowedError: errors.New("denied")})
	if err == nil || !strings.Contains(err.Error(), "owned table messages") {
		t.Fatalf("expected owned table rejection, got %v", err)
	}
}
