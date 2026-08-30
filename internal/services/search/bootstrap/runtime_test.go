package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	searchv1 "github.com/JekYUlll/Dipole/api/gen/go/search/v1"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
)

func TestSearchRuntimeComposesCoreScopeAndReadOnlyElasticsearch(t *testing.T) {
	var mutationRequests atomic.Int32
	elasticsearch := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodPost {
			mutationRequests.Add(1)
		}
		switch request.URL.Path {
		case "/_alias/dipole-messages-read,dipole-messages-write":
			_, _ = writer.Write([]byte(`{"dipole-messages-v2":{"aliases":{"dipole-messages-read":{},"dipole-messages-write":{"is_write_index":true}}}}`))
		case "/dipole-messages-v2/_mapping":
			_, _ = writer.Write([]byte(searchRuntimeMappingResponse))
		case "/dipole-messages-read/_search":
			_, _ = writer.Write([]byte(`{"hits":{"hits":[{"_source":{"message_uuid":"M1","conversation_key":"direct:U1:U2","message_seq":7,"revision":1,"sender_uuid":"U2","message_type":0,"content":"migration","sent_at":"2026-08-27T12:30:00Z","searchable":true,"payload_hash":"hash"}}]}}`))
		default:
			t.Fatalf("unexpected Elasticsearch request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer elasticsearch.Close()

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
	runtime, err := InitializeWithConfig(t.Context(), rpcCfg, config.Elasticsearch{
		Enabled: true, Address: elasticsearch.URL, IndexPrefix: "dipole", Shards: 1, Replicas: 0, RequestTimeoutSeconds: 2,
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
	if err != nil || len(documents) != 1 || documents[0].ConversationKey != "direct:U1:U2" {
		t.Fatalf("runtime Search: documents=%+v err=%v", documents, err)
	}
	if mutationRequests.Load() != 0 {
		t.Fatalf("Search runtime issued %d Elasticsearch mutation requests", mutationRequests.Load())
	}
	coreConnection, err := platformrpc.Dial(t.Context(), rpcCfg, rpcCfg.SearchTarget, grpcauth.Credentials{Service: "dipole-core", Secret: rpcCfg.SharedSecret})
	if err != nil {
		t.Fatalf("dial Search runtime as Core: %v", err)
	}
	t.Cleanup(func() { _ = coreConnection.Close() })
	coreSearch, err := searchgrpc.NewClientForService(searchv1.NewSearchServiceClient(coreConnection), "dipole-core")
	if err != nil {
		t.Fatalf("create Core Search client: %v", err)
	}
	coreDocuments, err := coreSearch.Search("U1", "migration", 10)
	if err != nil || len(coreDocuments) != 1 || coreDocuments[0].ConversationKey != "direct:U1:U2" {
		t.Fatalf("Core scoped Search: documents=%+v err=%v", coreDocuments, err)
	}
}

const searchRuntimeMappingResponse = `{
  "dipole-messages-v2": {
    "mappings": {
      "dynamic": "strict",
      "properties": {
        "message_uuid": {"type":"keyword"},
        "conversation_key": {"type":"keyword"},
        "message_seq": {"type":"long"},
        "revision": {"type":"long"},
        "sender_uuid": {"type":"keyword"},
        "message_type": {"type":"byte"},
        "content": {"type":"text"},
        "sent_at": {"type":"date"},
        "searchable": {"type":"boolean"},
        "payload_hash": {"type":"keyword","index":false}
      }
    }
  }
}`
