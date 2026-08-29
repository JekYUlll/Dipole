package coremysql_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

const conversationBenchmarkMembers = 1000

// BenchmarkConversationGroupProjection1000 compares the legacy member loop with
// the optional SQLC INSERT ... SELECT path on the same isolated MySQL schema.
// It is opt-in because it creates a database and requires a real MySQL server.
func BenchmarkConversationGroupProjection1000(b *testing.B) {
	db, _ := openContractDatabase(b)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		b.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		b.Fatalf("migrate benchmark database: %v", err)
	}
	repo, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		b.Fatalf("create conversation repository: %v", err)
	}

	b.Run("serial", func(b *testing.B) {
		members, groupUUID, conversationKey := seedConversationBenchmark(b, db, "serial")
		benchmarkConversationSerial(b, db, repo, members, groupUUID, conversationKey)
	})
	b.Run("batch", func(b *testing.B) {
		_, groupUUID, conversationKey := seedConversationBenchmark(b, db, "batch")
		benchmarkConversationBatch(b, db, repo, groupUUID, conversationKey)
	})
	b.Run("parallel-serial", func(b *testing.B) {
		members, groupUUID, conversationKey := seedConversationBenchmark(b, db, "pserial")
		benchmarkConversationParallelSerial(b, db, repo, members, groupUUID, conversationKey)
	})
	b.Run("parallel-batch", func(b *testing.B) {
		_, groupUUID, conversationKey := seedConversationBenchmark(b, db, "pbatch")
		benchmarkConversationParallelBatch(b, db, repo, groupUUID, conversationKey)
	})
}

func seedConversationBenchmark(b *testing.B, db *sql.DB, suffix string) ([]string, string, string) {
	b.Helper()
	groupUUID := fmt.Sprintf("G-bm-%s-%d", suffix, time.Now().UnixNano()%1000000000)
	conversationKey := model.GroupConversationKey(groupUUID)
	ownerUUID := fmt.Sprintf("U-bm-%s-owner", suffix)
	if _, err := db.Exec("INSERT INTO `groups` (uuid, name, owner_uuid, member_count, status) VALUES (?, ?, ?, ?, ?)", groupUUID, suffix, ownerUUID, conversationBenchmarkMembers, model.GroupStatusNormal); err != nil {
		b.Fatalf("insert benchmark group: %v", err)
	}
	members := make([]string, 0, conversationBenchmarkMembers)
	stmt, err := db.Prepare("INSERT INTO group_members (group_uuid, user_uuid, role, joined_at) VALUES (?, ?, 0, NOW(3))")
	if err != nil {
		b.Fatalf("prepare benchmark members: %v", err)
	}
	defer stmt.Close()
	for i := 0; i < conversationBenchmarkMembers; i++ {
		userUUID := fmt.Sprintf("U-bm-%s-%04d", suffix, i)
		members = append(members, userUUID)
		if _, err := stmt.Exec(groupUUID, userUUID); err != nil {
			b.Fatalf("insert benchmark member %d: %v", i, err)
		}
	}
	return members, groupUUID, conversationKey
}

func benchmarkConversationSerial(b *testing.B, db *sql.DB, repo *sqlcRepository.ConversationRepository, members []string, groupUUID, conversationKey string) {
	b.Helper()
	beforeWaits, beforeTime := innodbRowLockStatus(b, db)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		message := benchmarkConversationMessage(conversationKey, groupUUID, uint64(i+1), "M-serial")
		for memberIndex, userUUID := range members {
			increment := 1
			if memberIndex == 0 {
				increment = 0
			}
			if err := repo.UpsertGroupMessage(userUUID, groupUUID, message, increment); err != nil {
				b.Fatalf("serial projection: %v", err)
			}
		}
	}
	b.StopTimer()
	reportConversationBenchmark(b, db, beforeWaits, beforeTime, conversationKey)
}

func benchmarkConversationBatch(b *testing.B, db *sql.DB, repo *sqlcRepository.ConversationRepository, groupUUID, conversationKey string) {
	b.Helper()
	beforeWaits, beforeTime := innodbRowLockStatus(b, db)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := repo.UpsertGroupMessageBatch(groupUUID, benchmarkConversationMessage(conversationKey, groupUUID, uint64(i+1), "M-batch")); err != nil {
			b.Fatalf("batch projection: %v", err)
		}
	}
	b.StopTimer()
	reportConversationBenchmark(b, db, beforeWaits, beforeTime, conversationKey)
}

func benchmarkConversationParallelSerial(b *testing.B, db *sql.DB, repo *sqlcRepository.ConversationRepository, members []string, groupUUID, conversationKey string) {
	b.Helper()
	var sequence atomic.Uint64
	beforeWaits, beforeTime := innodbRowLockStatus(b, db)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			seq := sequence.Add(1)
			message := benchmarkConversationMessage(conversationKey, groupUUID, seq, "M-parallel-serial")
			for memberIndex, userUUID := range members {
				increment := 1
				if memberIndex == 0 {
					increment = 0
				}
				if err := repo.UpsertGroupMessage(userUUID, groupUUID, message, increment); err != nil {
					b.Errorf("parallel serial projection: %v", err)
					return
				}
			}
		}
	})
	b.StopTimer()
	reportConversationBenchmark(b, db, beforeWaits, beforeTime, conversationKey)
}

func benchmarkConversationParallelBatch(b *testing.B, db *sql.DB, repo *sqlcRepository.ConversationRepository, groupUUID, conversationKey string) {
	b.Helper()
	var sequence atomic.Uint64
	beforeWaits, beforeTime := innodbRowLockStatus(b, db)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			seq := sequence.Add(1)
			if err := repo.UpsertGroupMessageBatch(groupUUID, benchmarkConversationMessage(conversationKey, groupUUID, seq, "M-parallel-batch")); err != nil {
				b.Errorf("parallel batch projection: %v", err)
				return
			}
		}
	})
	b.StopTimer()
	reportConversationBenchmark(b, db, beforeWaits, beforeTime, conversationKey)
}

func benchmarkConversationMessage(conversationKey, groupUUID string, seq uint64, prefix string) *model.Message {
	return contractConversationMessage(fmt.Sprintf("%s-%d", prefix, seq), conversationKey, groupUUID, seq, model.MessageTargetGroup, model.MessageTypeText, "benchmark", time.Now().UTC())
}

func reportConversationBenchmark(b *testing.B, db *sql.DB, beforeWaits, beforeTime int64, conversationKey string) {
	b.Helper()
	afterWaits, afterTime := innodbRowLockStatus(b, db)
	b.ReportMetric(float64(afterWaits-beforeWaits), "lock_waits")
	b.ReportMetric(float64(afterTime-beforeTime), "lock_time_ms")
	var rows int
	var maxSeq sql.NullInt64
	if err := db.QueryRow("SELECT COUNT(*), MAX(last_message_seq) FROM conversations WHERE conversation_key = ?", conversationKey).Scan(&rows, &maxSeq); err != nil {
		b.Fatalf("verify benchmark projection: %v", err)
	}
	if rows != conversationBenchmarkMembers || !maxSeq.Valid || maxSeq.Int64 <= 0 {
		b.Fatalf("unexpected benchmark projection rows=%d max_seq=%v", rows, maxSeq)
	}
}

func innodbRowLockStatus(b *testing.B, db *sql.DB) (int64, int64) {
	b.Helper()
	rows, err := db.Query("SHOW GLOBAL STATUS WHERE Variable_name IN ('Innodb_row_lock_waits', 'Innodb_row_lock_time')")
	if err != nil {
		b.Fatalf("read InnoDB lock status: %v", err)
	}
	defer rows.Close()
	var waits, lockTime int64
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			b.Fatalf("scan InnoDB lock status: %v", err)
		}
		var parsed int64
		if _, err := fmt.Sscan(value, &parsed); err != nil {
			b.Fatalf("parse InnoDB lock status %s: %v", name, err)
		}
		switch name {
		case "Innodb_row_lock_waits":
			waits = parsed
		case "Innodb_row_lock_time":
			lockTime = parsed
		}
	}
	if err := rows.Err(); err != nil {
		b.Fatalf("iterate InnoDB lock status: %v", err)
	}
	return waits, lockTime
}
