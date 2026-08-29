package messagemysql_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
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
		firstInboxInserted: make(chan struct{}),
		releaseFirst:       make(chan struct{}),
	}
	repo, err := messagemysql.NewMessageRepository(&pausingTransactionStore{db: db, control: control})
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
	select {
	case err := <-secondDone:
		t.Fatalf("second transaction bypassed conversation sequence lock: %v", err)
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
	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("unexpected conversation sequences: first=%d second=%d", first.Seq, second.Seq)
	}
}

func TestSQLCConversationSequenceIsContinuousUnderConcurrency(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	repo, err := messagemysql.NewMessageRepository(store)
	if err != nil {
		t.Fatalf("create message repository: %v", err)
	}

	const messageCount = 24
	conversationKey := "direct:U-seq-a:U-seq-b"
	start := make(chan struct{})
	errorsCh := make(chan error, messageCount)
	var wg sync.WaitGroup
	for index := 0; index < messageCount; index++ {
		message := contractStoredMessage(fmt.Sprintf("seq-%02d", index), conversationKey, time.Now().UTC().Add(time.Duration(index)*time.Millisecond))
		wg.Add(1)
		go func(message *model.Message) {
			defer wg.Done()
			<-start
			errorsCh <- repo.CreateWithSync(message, nil)
		}(message)
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent message create: %v", err)
		}
	}

	rows, err := db.Query("SELECT seq FROM messages WHERE conversation_key = ? ORDER BY seq", conversationKey)
	if err != nil {
		t.Fatalf("query conversation sequences: %v", err)
	}
	defer rows.Close()
	var index uint64 = 1
	for rows.Next() {
		var sequence uint64
		if err := rows.Scan(&sequence); err != nil {
			t.Fatalf("scan conversation sequence: %v", err)
		}
		if sequence != index {
			t.Fatalf("sequence at position %d = %d", index, sequence)
		}
		index++
	}
	if index != messageCount+1 {
		t.Fatalf("sequence row count = %d, want %d", index-1, messageCount)
	}
}

func TestSQLCDeviceSyncCheckpointIsMonotonicUnderConcurrency(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	repo, err := sqlcRepository.NewSyncRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sync repository: %v", err)
	}

	const checkpointCount = 24
	userUUID := "U-checkpoint"
	for index := 1; index <= checkpointCount; index++ {
		if _, err := db.Exec(`INSERT INTO user_sync_inbox (user_uuid, message_uuid, conversation_key, message_seq)
			VALUES (?, ?, 'direct:checkpoint', ?)`, userUUID, fmt.Sprintf("M-checkpoint-%02d", index), index); err != nil {
			t.Fatalf("seed sync inbox: %v", err)
		}
	}

	start := make(chan struct{})
	errorsCh := make(chan error, checkpointCount)
	var wg sync.WaitGroup
	for sequence := 1; sequence <= checkpointCount; sequence++ {
		wg.Add(1)
		go func(sequence uint64) {
			defer wg.Done()
			<-start
			errorsCh <- repo.AdvanceDeviceSyncCheckpoint(userUUID, "web-a", sequence)
		}(uint64(sequence))
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("advance concurrent checkpoint: %v", err)
		}
	}
	checkpoint, err := repo.GetDeviceCheckpoint(userUUID, "web-a")
	if err != nil || checkpoint == nil || checkpoint.SyncSeq != checkpointCount {
		t.Fatalf("final checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	if err := repo.AdvanceDeviceSyncCheckpoint(userUUID, "mobile-b", 7); err != nil {
		t.Fatalf("advance second device: %v", err)
	}
	other, err := repo.GetDeviceCheckpoint(userUUID, "mobile-b")
	if err != nil || other == nil || other.SyncSeq != 7 {
		t.Fatalf("second device checkpoint: checkpoint=%+v err=%v", other, err)
	}
}

type syncCommitControl struct {
	firstInboxInserted chan struct{}
	releaseFirst       chan struct{}
	firstOnce          sync.Once
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
