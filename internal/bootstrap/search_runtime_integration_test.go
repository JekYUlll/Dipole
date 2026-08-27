package bootstrap

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	elasticsearchdata "github.com/JekYUlll/Dipole/internal/data/elasticsearch"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestSearchRuntimeElasticsearchContract(t *testing.T) {
	address := os.Getenv("DIPOLE_TEST_ELASTICSEARCH_URL")
	if address == "" {
		t.Skip("DIPOLE_TEST_ELASTICSEARCH_URL is required for Search runtime integration tests")
	}
	prefix := "dipole-search-service-contract"
	client := &http.Client{Timeout: 5 * time.Second}
	index, err := elasticsearchdata.NewIndex(elasticsearchdata.Config{
		Address: address, IndexPrefix: prefix, Shards: 1, Replicas: 0, HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("create Elasticsearch index: %v", err)
	}
	if err := index.Bootstrap(t.Context()); err != nil {
		t.Fatalf("bootstrap Elasticsearch index: %v", err)
	}
	for _, document := range []*model.MessageSearchDocument{
		{MessageUUID: "M-visible", ConversationKey: "direct:U1:U2", MessageSeq: 1, Revision: 1, SenderUUID: "U2", MessageType: model.MessageTypeText, Content: "migration visible", SentAt: time.Unix(1, 0)},
		{MessageUUID: "M-hidden", ConversationKey: "group:G-hidden", MessageSeq: 1, Revision: 1, SenderUUID: "U3", MessageType: model.MessageTypeText, Content: "migration hidden", SentAt: time.Unix(2, 0)},
	} {
		if err := index.Apply(&model.MessageSearchMutation{Type: model.MessageSearchMutationUpsert, MessageUUID: document.MessageUUID, Revision: 1, Document: document}); err != nil {
			t.Fatalf("index document %s: %v", document.MessageUUID, err)
		}
	}
	target, err := index.PhysicalTarget(t.Context(), index.PhysicalIndex())
	if err != nil {
		t.Fatalf("open physical target: %v", err)
	}
	if err := target.Refresh(t.Context()); err != nil {
		t.Fatalf("refresh physical target: %v", err)
	}

	rpcCfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0",
		SearchListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2, ShutdownTimeoutSeconds: 2,
	}
	coreServer, err := NewCoreRPCServer(rpcCfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core rpc: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		coreServer.Close(ctx)
	})
	rpcCfg.CoreTarget = coreServer.Address()
	runtime, err := initializeSearchService(t.Context(), rpcCfg, config.Elasticsearch{
		Enabled: true, Address: address, IndexPrefix: prefix, Shards: 1, Replicas: 0, RequestTimeoutSeconds: 5,
	}, config.Metrics{})
	if err != nil {
		t.Fatalf("initialize Search runtime: %v", err)
	}
	t.Cleanup(runtime.Close)
	rpcCfg.SearchTarget = runtime.Address()
	search, connection, err := DialSearchApplication(t.Context(), rpcCfg)
	if err != nil {
		t.Fatalf("dial Search runtime: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	documents, err := search.Search("U1", "migration", 10)
	if err != nil || len(documents) != 1 || documents[0].MessageUUID != "M-visible" {
		t.Fatalf("scoped real Search: documents=%+v err=%v", documents, err)
	}
}
