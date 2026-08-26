package elasticsearch

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestBootstrapCreatesVersionedIndexWithReadAndWriteAliases(t *testing.T) {
	var createBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodHead:
			writer.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			if request.URL.Path != "/dipole-messages-v1" {
				t.Fatalf("unexpected create path %s", request.URL.Path)
			}
			if err := json.NewDecoder(request.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"acknowledged":true}`))
		default:
			t.Fatalf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()

	index := testIndex(t, server.URL)
	if err := index.Bootstrap(t.Context()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}
	if index.PhysicalIndex() != "dipole-messages-v1" || index.ReadAlias() != "dipole-messages-read" || index.WriteAlias() != "dipole-messages-write" {
		t.Fatalf("unexpected index names: %s %s %s", index.PhysicalIndex(), index.ReadAlias(), index.WriteAlias())
	}
	aliases := createBody["aliases"].(map[string]any)
	writeAlias := aliases[index.WriteAlias()].(map[string]any)
	if writeAlias["is_write_index"] != true {
		t.Fatalf("write alias must be explicit: %#v", aliases)
	}
	mappings := createBody["mappings"].(map[string]any)
	if mappings["dynamic"] != "strict" {
		t.Fatalf("expected strict mappings: %#v", mappings)
	}
}

func TestBootstrapValidatesExistingAliasOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.WriteHeader(http.StatusOK)
			return
		}
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected validation method %s", request.Method)
		}
		switch request.URL.Path {
		case "/dipole-messages-v1/_mapping":
			_, _ = writer.Write([]byte(validExistingMappingResponse()))
		case "/dipole-messages-v1/_alias/dipole-messages-read,dipole-messages-write":
			_, _ = writer.Write([]byte(`{"dipole-messages-v1":{"aliases":{"dipole-messages-read":{},"dipole-messages-write":{"is_write_index":true}}}}`))
		default:
			t.Fatalf("unexpected validation path %s", request.URL.Path)
		}
	}))
	defer server.Close()

	if err := testIndex(t, server.URL).Bootstrap(t.Context()); err != nil {
		t.Fatalf("validate existing index: %v", err)
	}
}

func TestBootstrapRejectsExistingMappingDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.WriteHeader(http.StatusOK)
			return
		}
		_, _ = writer.Write([]byte(strings.Replace(validExistingMappingResponse(), `"content":{"type":"text"}`, `"content":{"type":"keyword"}`, 1)))
	}))
	defer server.Close()

	err := testIndex(t, server.URL).Bootstrap(t.Context())
	if err == nil || !strings.Contains(err.Error(), "field content must use type text") {
		t.Fatalf("expected mapping drift to fail readiness, got %v", err)
	}
}

func TestSwitchAliasesUsesOneAtomicActionRequest(t *testing.T) {
	var body struct {
		Actions []map[string]map[string]any `json:"actions"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/_alias/dipole-messages-read,dipole-messages-write" && len(body.Actions) == 0 {
			_, _ = writer.Write([]byte(validAliasResponse("dipole-messages-v1")))
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/dipole-messages-v2/_mapping" {
			_, _ = writer.Write([]byte(strings.Replace(validExistingMappingResponse(), "dipole-messages-v1", "dipole-messages-v2", 1)))
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/_alias/dipole-messages-read,dipole-messages-write" && len(body.Actions) > 0 {
			_, _ = writer.Write([]byte(validAliasResponse("dipole-messages-v2")))
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/_aliases" {
			t.Fatalf("unexpected alias switch request %s %s", request.Method, request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode alias actions: %v", err)
		}
		_, _ = writer.Write([]byte(`{"acknowledged":true}`))
	}))
	defer server.Close()

	index := testIndex(t, server.URL)
	if err := index.SwitchAliases(t.Context(), "dipole-messages-v1", "dipole-messages-v2"); err != nil {
		t.Fatalf("switch aliases: %v", err)
	}
	if len(body.Actions) != 4 || body.Actions[0]["remove"]["must_exist"] != true || body.Actions[3]["add"]["is_write_index"] != true {
		t.Fatalf("unexpected alias actions: %#v", body.Actions)
	}
}

func TestSwitchAliasesRejectsSplitOwnershipBeforeMutation(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/_alias/dipole-messages-read,dipole-messages-write" {
			_, _ = writer.Write([]byte(`{"dipole-messages-v1":{"aliases":{"dipole-messages-read":{},"dipole-messages-write":{"is_write_index":true}}},"rogue":{"aliases":{"dipole-messages-read":{}}}}`))
			return
		}
		if request.Method == http.MethodPost {
			posts++
		}
	}))
	defer server.Close()

	err := testIndex(t, server.URL).SwitchAliases(t.Context(), "dipole-messages-v1", "dipole-messages-v2")
	if err == nil || posts != 0 || !strings.Contains(err.Error(), "one physical owner") {
		t.Fatalf("expected split ownership rejection: posts=%d err=%v", posts, err)
	}
}

func TestSwitchAliasesCompensatesPostValidationFailure(t *testing.T) {
	owner := "dipole-messages-v1"
	posts := 0
	failNewValidation := true
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/_alias/dipole-messages-read,dipole-messages-write":
			if owner == "dipole-messages-v2" && failNewValidation {
				failNewValidation = false
				writer.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = writer.Write([]byte(validAliasResponse(owner)))
		case request.Method == http.MethodGet && request.URL.Path == "/dipole-messages-v2/_mapping":
			_, _ = writer.Write([]byte(strings.Replace(validExistingMappingResponse(), "dipole-messages-v1", "dipole-messages-v2", 1)))
		case request.Method == http.MethodPost && request.URL.Path == "/_aliases":
			posts++
			if posts == 1 {
				owner = "dipole-messages-v2"
			} else {
				owner = "dipole-messages-v1"
			}
			_, _ = writer.Write([]byte(`{"acknowledged":true}`))
		default:
			t.Fatalf("unexpected compensation request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()

	err := testIndex(t, server.URL).SwitchAliases(t.Context(), "dipole-messages-v1", "dipole-messages-v2")
	if err == nil || posts != 2 || owner != "dipole-messages-v1" || !strings.Contains(err.Error(), "restored") {
		t.Fatalf("expected compensated post-check: owner=%s posts=%d err=%v", owner, posts, err)
	}
}

func TestApplyClassifiesReplayStaleAndDivergentRevision(t *testing.T) {
	var current model.MessageSearchState
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPut:
			if request.URL.Query().Get("require_alias") != "true" || request.URL.Query().Get("version_type") != "external" {
				t.Fatalf("missing external alias guards: %s", request.URL.RawQuery)
			}
			var candidate model.MessageSearchState
			if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
				t.Fatalf("decode candidate: %v", err)
			}
			if current.MessageUUID == "" || candidate.Revision > current.Revision {
				current = candidate
				writer.WriteHeader(http.StatusCreated)
				return
			}
			writer.WriteHeader(http.StatusConflict)
		case http.MethodGet:
			_ = json.NewEncoder(writer).Encode(map[string]any{"_source": current})
		default:
			t.Fatalf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()

	index := testIndex(t, server.URL)
	document := validSearchDocument()
	document.Revision = 2
	if err := index.Apply(upsertMutation(document)); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	if err := index.Apply(upsertMutation(document)); err != nil {
		t.Fatalf("identical replay: %v", err)
	}
	stale := *document
	stale.Revision = 1
	stale.Content = "stale"
	if err := index.Apply(upsertMutation(&stale)); err != nil {
		t.Fatalf("stale revision should be a no-op: %v", err)
	}
	conflict := *document
	conflict.Content = "divergent"
	if err := index.Apply(upsertMutation(&conflict)); !errors.Is(err, ErrProjectionConflict) {
		t.Fatalf("expected divergent equal revision conflict, got %v", err)
	}
}

func TestApplyRejectsValuesOutsideElasticsearchLong(t *testing.T) {
	index := &Index{}
	document := validSearchDocument()
	document.Revision = uint64(math.MaxInt64) + 1
	if err := index.Apply(upsertMutation(document)); err == nil || !strings.Contains(err.Error(), "must fit storage long") {
		t.Fatalf("expected oversized revision to fail before IO, got %v", err)
	}
	document.Revision = 1
	document.MessageSeq = uint64(math.MaxInt64) + 1
	if err := index.Apply(upsertMutation(document)); err == nil || !strings.Contains(err.Error(), "sequence must fit storage long") {
		t.Fatalf("expected oversized sequence to fail before IO, got %v", err)
	}
}

func TestPhysicalTargetCreatesWithoutAliasesAndSupportsReconciliation(t *testing.T) {
	var createdAliases any
	state := model.MessageSearchState{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodHead && request.URL.Path == "/dipole-messages-v1-build-a":
			writer.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodPut && request.URL.Path == "/dipole-messages-v1-build-a":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode physical index create: %v", err)
			}
			createdAliases = body["aliases"]
			_, _ = writer.Write([]byte(`{"acknowledged":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/dipole-messages-v1-build-a/_doc/M1":
			if request.URL.Query().Get("require_alias") != "" || request.URL.Query().Get("version_type") != "external" {
				t.Fatalf("unexpected physical write query: %s", request.URL.RawQuery)
			}
			if err := json.NewDecoder(request.Body).Decode(&state); err != nil {
				t.Fatalf("decode physical state: %v", err)
			}
			writer.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && request.URL.Path == "/dipole-messages-v1-build-a/_doc/M1":
			_ = json.NewEncoder(writer).Encode(map[string]any{"_source": state})
		case request.Method == http.MethodPost && request.URL.Path == "/dipole-messages-v1-build-a/_count":
			_, _ = writer.Write([]byte(`{"count":1}`))
		case request.Method == http.MethodPost && request.URL.Path == "/dipole-messages-v1-build-a/_refresh":
			_, _ = writer.Write([]byte(`{"_shards":{"successful":1}}`))
		default:
			t.Fatalf("unexpected physical target request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()

	index := testIndex(t, server.URL)
	target, err := index.CreatePhysicalTarget(t.Context(), "dipole-messages-v1-build-a")
	if err != nil {
		t.Fatalf("create physical target: %v", err)
	}
	if createdAliases != nil {
		t.Fatalf("build index must not bind production aliases: %#v", createdAliases)
	}
	document := validSearchDocument()
	document.Revision = 1
	if err := target.Apply(upsertMutation(document)); err != nil {
		t.Fatalf("apply physical mutation: %v", err)
	}
	if err := target.Refresh(t.Context()); err != nil {
		t.Fatalf("refresh physical target: %v", err)
	}
	actual, found, err := target.Lookup(t.Context(), "M1")
	if err != nil || !found || actual.PayloadHash != state.PayloadHash {
		t.Fatalf("lookup state=%+v found=%v err=%v", actual, found, err)
	}
	count, err := target.Count(t.Context())
	if err != nil || count != 1 {
		t.Fatalf("target count=%d err=%v", count, err)
	}
}

func TestSearchRequiresAndTransmitsConversationScope(t *testing.T) {
	var queryBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/dipole-messages-read/_search" {
			t.Fatalf("unexpected search request %s %s", request.Method, request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&queryBody); err != nil {
			t.Fatalf("decode search query: %v", err)
		}
		_, _ = writer.Write([]byte(`{"hits":{"hits":[{"_source":{"message_uuid":"M1","conversation_key":"direct:U1:U2","message_seq":7,"revision":2,"sender_uuid":"U1","message_type":0,"content":"migration approved","sent_at":"2026-08-27T08:00:00.000Z","searchable":true,"payload_hash":"abc"}}]}}`))
	}))
	defer server.Close()

	index := testIndex(t, server.URL)
	if _, err := index.Search(model.MessageSearchQuery{Text: "migration"}); err == nil {
		t.Fatal("expected empty conversation scope to fail closed")
	}
	results, err := index.Search(model.MessageSearchQuery{
		ConversationKeys: []string{" group:G2 ", "direct:U1:U2", "group:G2"}, Text: " migration ", Limit: 500,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].MessageUUID != "M1" || results[0].Revision != 2 || results[0].SentAt.IsZero() {
		t.Fatalf("unexpected search results: %+v", results)
	}
	if queryBody["size"].(float64) != 100 {
		t.Fatalf("expected capped search size: %#v", queryBody)
	}
	boolQuery := queryBody["query"].(map[string]any)["bool"].(map[string]any)
	filter := boolQuery["filter"].([]any)[0].(map[string]any)
	keys := filter["terms"].(map[string]any)["conversation_key"].([]any)
	want := []any{"direct:U1:U2", "group:G2"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("unexpected conversation filter: got=%v want=%v", keys, want)
	}
	must := boolQuery["must"].([]any)[0].(map[string]any)
	if !strings.Contains(must["match"].(map[string]any)["content"].(map[string]any)["query"].(string), "migration") {
		t.Fatalf("unexpected text query: %#v", must)
	}
}

func testIndex(t *testing.T, address string) *Index {
	t.Helper()
	index, err := NewIndex(Config{Address: address, IndexPrefix: "dipole", Shards: 1, Replicas: 0, HTTPClient: http.DefaultClient})
	if err != nil {
		t.Fatalf("create index: %v", err)
	}
	return index
}

func validSearchDocument() *model.MessageSearchDocument {
	return &model.MessageSearchDocument{
		MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 7,
		SenderUUID: "U1", Content: "migration approved", SentAt: time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC),
	}
}

func upsertMutation(document *model.MessageSearchDocument) *model.MessageSearchMutation {
	return &model.MessageSearchMutation{
		Type: model.MessageSearchMutationUpsert, MessageUUID: document.MessageUUID,
		Revision: document.Revision, Document: document,
	}
}

func validExistingMappingResponse() string {
	return `{"dipole-messages-v1":{"mappings":{"dynamic":"strict","properties":{"message_uuid":{"type":"keyword"},"conversation_key":{"type":"keyword"},"message_seq":{"type":"long"},"revision":{"type":"long"},"sender_uuid":{"type":"keyword"},"message_type":{"type":"byte"},"content":{"type":"text"},"sent_at":{"type":"date"},"searchable":{"type":"boolean"},"payload_hash":{"type":"keyword","index":false}}}}}`
}

func validAliasResponse(index string) string {
	return `{"` + index + `":{"aliases":{"dipole-messages-read":{},"dipole-messages-write":{"is_write_index":true}}}}`
}
