package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestIndexContract(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("DIPOLE_TEST_ELASTICSEARCH_URL"))
	if address == "" {
		t.Skip("DIPOLE_TEST_ELASTICSEARCH_URL is required for Elasticsearch integration tests")
	}
	prefix := fmt.Sprintf("dipole-contract-%d", time.Now().UnixNano())
	index, err := NewIndex(Config{
		Address: address, IndexPrefix: prefix, Shards: 1, Replicas: 0,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	})
	if err != nil {
		t.Fatalf("create Elasticsearch index: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Cleanup(func() {
		_, _, _ = index.request(context.Background(), http.MethodDelete, "/"+index.PhysicalIndex(), nil)
	})

	if err := index.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap Elasticsearch index: %v", err)
	}
	if err := index.Bootstrap(ctx); err != nil {
		t.Fatalf("repeat Elasticsearch bootstrap: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	document := &model.MessageSearchDocument{
		MessageUUID: "M-CONTRACT", ConversationKey: "direct:U1:U2", MessageSeq: 1, Revision: 1,
		SenderUUID: "U1", Content: "database migration planned", SentAt: now,
	}
	if err := index.Upsert(document); err != nil {
		t.Fatalf("upsert first revision: %v", err)
	}
	if err := index.Upsert(document); err != nil {
		t.Fatalf("replay first revision: %v", err)
	}
	document.Revision = 2
	document.Content = "database migration approved"
	if err := index.Upsert(document); err != nil {
		t.Fatalf("upsert second revision: %v", err)
	}
	stale := *document
	stale.Revision = 1
	stale.Content = "database migration stale"
	if err := index.Upsert(&stale); err != nil {
		t.Fatalf("ignore stale revision: %v", err)
	}
	status, response, err := index.request(ctx, http.MethodPost, "/"+index.PhysicalIndex()+"/_refresh", nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("refresh Elasticsearch index: status=%d body=%s err=%v", status, response, err)
	}

	results, err := index.Search(model.MessageSearchQuery{
		ConversationKeys: []string{"direct:U1:U2"}, Text: "migration", Limit: 10,
	})
	if err != nil {
		t.Fatalf("search Elasticsearch index: %v", err)
	}
	if len(results) != 1 || results[0].Revision != 2 || results[0].Content != "database migration approved" {
		t.Fatalf("unexpected search result: %+v", results)
	}
	hidden, err := index.Search(model.MessageSearchQuery{
		ConversationKeys: []string{"group:G-hidden"}, Text: "migration", Limit: 10,
	})
	if err != nil || len(hidden) != 0 {
		t.Fatalf("conversation scope leaked: results=%+v err=%v", hidden, err)
	}
}
