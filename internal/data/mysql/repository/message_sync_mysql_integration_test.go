package repository_test

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestSQLCMessageSyncSequenceFollowsCommitOrder(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	control := &syncCommitControl{
		firstInboxInserted:    make(chan struct{}),
		secondMessageInserted: make(chan struct{}),
		releaseFirst:          make(chan struct{}),
	}
	repo, err := sqlcRepository.NewMessageRepository(&pausingTransactionStore{db: db, control: control})
	if err != nil {
		t.Fatalf("create sqlc message repository: %v", err)
	}
	conversationKey := "direct:U-sync-a:U-sync-b"
	first := contractStoredMessage("sqlc-commit-1", conversationKey, time.Now().UTC())
	second := contractStoredMessage("sqlc-commit-2", conversationKey, time.Now().UTC().Add(time.Second))
	first.TargetUUID, second.TargetUUID = "U-sync-b", "U-sync-b"
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { firstDone <- repo.CreateWithSync(first, []string{"U-sync-b"}) }()
	waitForContractSignal(t, control.firstInboxInserted, "first inbox insert")
	go func() { secondDone <- repo.CreateWithSync(second, []string{"U-sync-b"}) }()
	waitForContractSignal(t, control.secondMessageInserted, "second message insert")
	select {
	case err := <-secondDone:
		t.Fatalf("second transaction bypassed user sync lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(control.releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("commit first message: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("commit second message: %v", err)
	}
	rows, err := generated.New(db).ListUserSyncInboxAfter(context.Background(), generated.ListUserSyncInboxAfterParams{UserUuid: "U-sync-b", Limit: 10})
	if err != nil {
		t.Fatalf("list sync inbox: %v", err)
	}
	if len(rows) != 2 || rows[0].MessageUuid != first.UUID || rows[1].MessageUuid != second.UUID {
		t.Fatalf("unexpected commit-ordered inbox: %+v", rows)
	}
}

type syncCommitControl struct {
	firstInboxInserted    chan struct{}
	secondMessageInserted chan struct{}
	releaseFirst          chan struct{}
	firstOnce             sync.Once
	secondOnce            sync.Once
}

type pausingTransactionStore struct {
	db      *sql.DB
	control *syncCommitControl
}

func (s *pausingTransactionStore) Queries() *generated.Queries { return generated.New(s.db) }

func (s *pausingTransactionStore) WithinTx(ctx context.Context, options *sql.TxOptions, fn func(*generated.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(generated.New(&pausingDBTX{Tx: tx, control: s.control})); err != nil {
		return err
	}
	return tx.Commit()
}

type pausingDBTX struct {
	*sql.Tx
	control *syncCommitControl
}

func (db *pausingDBTX) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := db.Tx.ExecContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	if strings.Contains(query, "INSERT INTO messages") && len(args) > 0 && args[0] == "M-sqlc-commit-2" {
		db.control.secondOnce.Do(func() { close(db.control.secondMessageInserted) })
	}
	if strings.Contains(query, "INSERT INTO user_sync_inbox") && len(args) > 1 && args[1] == "M-sqlc-commit-1" {
		db.control.firstOnce.Do(func() { close(db.control.firstInboxInserted) })
		<-db.control.releaseFirst
	}
	return result, nil
}

func waitForContractSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}
