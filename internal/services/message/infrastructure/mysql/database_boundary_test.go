package messagemysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type databasePermissionProbeStub struct {
	allowedError     error
	coreAllowed      string
	denyInbox        bool
	forbiddenAllowed string
	queries          []string
}

func (s *databasePermissionProbeStub) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	s.queries = append(s.queries, query)
	for _, probe := range messageForbiddenPermissionProbes {
		if query == probe.query {
			if probe.table+":"+probe.verb == s.forbiddenAllowed {
				return nil, nil
			}
			return nil, &mysqlDriver.MySQLError{Number: 1142, Message: probe.verb + " denied"}
		}
	}
	for _, table := range coreOwnedTables {
		if strings.Contains(query, "`"+table+"`") {
			if table == s.coreAllowed {
				return nil, nil
			}
			return nil, &mysqlDriver.MySQLError{Number: 1142, Message: "SELECT denied"}
		}
	}
	for _, table := range messageAtomicInboxTables {
		if strings.Contains(query, "`"+table+"`") {
			if table == s.coreAllowed {
				return nil, nil
			}
			if s.denyInbox {
				return nil, &mysqlDriver.MySQLError{Number: 1142, Message: "SELECT denied"}
			}
			if strings.HasPrefix(query, "DELETE") {
				return nil, &mysqlDriver.MySQLError{Number: 1142, Message: "DELETE denied"}
			}
			return nil, s.allowedError
		}
	}
	return nil, s.allowedError
}

func TestVerifyMessageDatabaseBoundaryAcceptsLeastPrivilegeAccount(t *testing.T) {
	probe := &databasePermissionProbeStub{}
	if err := VerifyDatabaseBoundary(context.Background(), probe, true); err != nil {
		t.Fatalf("verify message database boundary: %v", err)
	}
	wantQueries := len(messageRequiredPermissionProbes) + len(messageAtomicInboxPermissionProbes) + len(messageDeniedPermissionProbes) + len(messageForbiddenPermissionProbes) + 2
	if len(probe.queries) != wantQueries {
		t.Fatalf("permission probes = %d, want %d", len(probe.queries), wantQueries)
	}
}

func TestVerifyMessageDatabaseBoundaryRejectsForbiddenOwnedOperation(t *testing.T) {
	err := VerifyDatabaseBoundary(context.Background(), &databasePermissionProbeStub{forbiddenAllowed: "schema_migrations:INSERT"}, true)
	if err == nil || !strings.Contains(err.Error(), "forbidden INSERT on schema_migrations") {
		t.Fatalf("expected forbidden schema write rejection, got %v", err)
	}
}

func TestVerifyMessageDatabaseBoundaryRejectsCoreTableAccess(t *testing.T) {
	err := VerifyDatabaseBoundary(context.Background(), &databasePermissionProbeStub{coreAllowed: "users"}, true)
	if err == nil || !strings.Contains(err.Error(), "forbidden SELECT on users") {
		t.Fatalf("expected users access rejection, got %v", err)
	}
}

func TestVerifyMessageDatabaseBoundaryRejectsMissingOwnedTableAccess(t *testing.T) {
	err := VerifyDatabaseBoundary(context.Background(), &databasePermissionProbeStub{allowedError: errors.New("denied")}, true)
	if err == nil || !strings.Contains(err.Error(), "lacks SELECT on messages") {
		t.Fatalf("expected owned table rejection, got %v", err)
	}
}

func TestVerifyMessageDatabaseBoundaryProjectorModeRejectsInboxAccess(t *testing.T) {
	err := VerifyDatabaseBoundary(context.Background(), &databasePermissionProbeStub{coreAllowed: "user_sync_inbox"}, false)
	if err == nil || !strings.Contains(err.Error(), "forbidden SELECT on user_sync_inbox") {
		t.Fatalf("expected Inbox access rejection, got %v", err)
	}
}

func TestVerifyMessageDatabaseBoundaryAcceptsProjectorAccountWithoutInboxAccess(t *testing.T) {
	if err := VerifyDatabaseBoundary(context.Background(), &databasePermissionProbeStub{denyInbox: true}, false); err != nil {
		t.Fatalf("verify projector account: %v", err)
	}
}
