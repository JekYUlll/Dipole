package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type syncDatabasePermissionProbeStub struct {
	requiredError error
	allowDenied   string
	queries       []string
}

func (s *syncDatabasePermissionProbeStub) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	s.queries = append(s.queries, query)
	for _, probe := range syncDeniedPermissionProbes {
		if query == probe.query {
			if probe.table+":"+probe.verb == s.allowDenied {
				return nil, nil
			}
			return nil, &mysqlDriver.MySQLError{Number: 1142, Message: probe.verb + " denied"}
		}
	}
	return nil, s.requiredError
}

func TestVerifySyncDatabaseBoundaryAcceptsLeastPrivilegeAccount(t *testing.T) {
	probe := &syncDatabasePermissionProbeStub{}
	if err := verifySyncDatabaseBoundary(context.Background(), probe); err != nil {
		t.Fatalf("verify Sync database boundary: %v", err)
	}
	if got, want := len(probe.queries), len(syncRequiredPermissionProbes)+len(syncDeniedPermissionProbes); got != want {
		t.Fatalf("permission probes = %d, want %d", got, want)
	}
}

func TestVerifySyncDatabaseBoundaryRejectsForbiddenWrite(t *testing.T) {
	err := verifySyncDatabaseBoundary(context.Background(), &syncDatabasePermissionProbeStub{allowDenied: "messages:INSERT"})
	if err == nil || !strings.Contains(err.Error(), "forbidden INSERT on messages") {
		t.Fatalf("expected Message write rejection, got %v", err)
	}
}

func TestVerifySyncDatabaseBoundaryRejectsMissingRequiredWrite(t *testing.T) {
	err := verifySyncDatabaseBoundary(context.Background(), &syncDatabasePermissionProbeStub{requiredError: errors.New("denied")})
	if err == nil || !strings.Contains(err.Error(), "lacks SELECT on messages") {
		t.Fatalf("expected required permission rejection, got %v", err)
	}
}
