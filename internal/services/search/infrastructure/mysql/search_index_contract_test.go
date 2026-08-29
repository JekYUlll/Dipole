package searchmysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/search/infrastructure/mysql"
)

func TestSearchIndexRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	index, err := sqlcRepository.NewSearchIndexRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create search index: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	documents := []*model.MessageSearchDocument{
		{MessageUUID: "MS-1", ConversationKey: "direct:U1:U2", MessageSeq: 1, Revision: 1, SenderUUID: "U1", Content: "database migration approved", SentAt: now},
		{MessageUUID: "MS-2", ConversationKey: "direct:U1:U2", MessageSeq: 2, Revision: 1, SenderUUID: "U2", Content: "database migration delayed", SentAt: now.Add(time.Second)},
		{MessageUUID: "MS-3", ConversationKey: "group:G-hidden", MessageSeq: 1, Revision: 1, SenderUUID: "U3", Content: "database migration secret", SentAt: now.Add(2 * time.Second)},
	}
	for _, document := range documents {
		if err := index.Apply(searchUpsert(document)); err != nil {
			t.Fatalf("upsert document: %v", err)
		}
	}
	documents[1].Revision = 2
	documents[1].Content = "database migration completed"
	if err := index.Apply(searchUpsert(documents[1])); err != nil {
		t.Fatalf("idempotent update: %v", err)
	}
	conflict := *documents[1]
	conflict.Content = "same revision divergent payload"
	if err := index.Apply(searchUpsert(&conflict)); !errors.Is(err, model.ErrMessageSearchMutationConflict) {
		t.Fatalf("expected equal revision conflict, got %v", err)
	}

	results, err := index.Search(model.MessageSearchQuery{ConversationKeys: []string{" direct:U1:U2 ", "direct:U1:U2"}, Text: "MIGRATION", Limit: 10})
	if err != nil || len(results) != 2 || results[0].MessageUUID != "MS-2" || results[0].Content != "database migration completed" {
		t.Fatalf("scoped ordered search: results=%+v err=%v", results, err)
	}
	if _, err := index.Search(model.MessageSearchQuery{Text: "migration", Limit: 10}); err == nil {
		t.Fatal("expected empty conversation scope to fail closed")
	}
	percent := &model.MessageSearchDocument{MessageUUID: "MS-4", ConversationKey: "direct:U1:U2", MessageSeq: 4, Revision: 1, SenderUUID: "U1", Content: "progress reached 100%", SentAt: now.Add(3 * time.Second)}
	if err := index.Apply(searchUpsert(percent)); err != nil {
		t.Fatalf("upsert percent document: %v", err)
	}
	results, err = index.Search(model.MessageSearchQuery{ConversationKeys: []string{"direct:U1:U2"}, Text: "%", Limit: 10})
	if err != nil || len(results) != 1 || results[0].MessageUUID != percent.MessageUUID {
		t.Fatalf("literal wildcard search: results=%+v err=%v", results, err)
	}
	tombstone := &model.MessageSearchMutation{Type: model.MessageSearchMutationTombstone, MessageUUID: "MS-2", Revision: 3}
	if err := index.Apply(tombstone); err != nil {
		t.Fatalf("tombstone document: %v", err)
	}
	stale := *documents[1]
	stale.Content = "database migration resurrected"
	if err := index.Apply(searchUpsert(&stale)); err != nil {
		t.Fatalf("stale upsert after tombstone: %v", err)
	}
	results, err = index.Search(model.MessageSearchQuery{ConversationKeys: []string{"direct:U1:U2"}, Text: "migration", Limit: 10})
	if err != nil || len(results) != 1 || results[0].MessageUUID != "MS-1" {
		t.Fatalf("search after delete: results=%+v err=%v", results, err)
	}
}

func searchUpsert(document *model.MessageSearchDocument) *model.MessageSearchMutation {
	return &model.MessageSearchMutation{
		Type: model.MessageSearchMutationUpsert, MessageUUID: document.MessageUUID,
		Revision: document.Revision, Document: document,
	}
}
