package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

const mappingVersion = "v1"

var (
	//go:embed schema/message_search_v1.json
	messageSearchMapping json.RawMessage

	ErrProjectionConflict = errors.New("Elasticsearch search projection conflict")
	indexPrefixPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

type Config struct {
	Address     string
	IndexPrefix string
	Shards      int
	Replicas    int
	HTTPClient  *http.Client
}

type Index struct {
	baseURL       *url.URL
	client        *http.Client
	physicalIndex string
	readAlias     string
	writeAlias    string
	shards        int
	replicas      int
}

var _ application.SearchIndex = (*Index)(nil)

type searchDocument struct {
	MessageUUID     string `json:"message_uuid"`
	ConversationKey string `json:"conversation_key"`
	MessageSeq      uint64 `json:"message_seq"`
	Revision        uint64 `json:"revision"`
	SenderUUID      string `json:"sender_uuid"`
	MessageType     int8   `json:"message_type"`
	Content         string `json:"content"`
	SentAt          string `json:"sent_at"`
	PayloadHash     string `json:"payload_hash"`
}

func NewIndex(config Config) (*Index, error) {
	baseURL, err := url.Parse(strings.TrimSpace(config.Address))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, errors.New("Elasticsearch address must be an absolute URL")
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, errors.New("Elasticsearch address must use http or https")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	prefix := strings.TrimSpace(config.IndexPrefix)
	if !indexPrefixPattern.MatchString(prefix) {
		return nil, errors.New("Elasticsearch index prefix is invalid")
	}
	if config.Shards <= 0 {
		return nil, errors.New("Elasticsearch shard count must be positive")
	}
	if config.Replicas < 0 {
		return nil, errors.New("Elasticsearch replica count cannot be negative")
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Index{
		baseURL: baseURL, client: client, shards: config.Shards, replicas: config.Replicas,
		physicalIndex: prefix + "-messages-" + mappingVersion,
		readAlias:     prefix + "-messages-read", writeAlias: prefix + "-messages-write",
	}, nil
}

func (i *Index) PhysicalIndex() string { return i.physicalIndex }
func (i *Index) ReadAlias() string     { return i.readAlias }
func (i *Index) WriteAlias() string    { return i.writeAlias }

func (i *Index) Bootstrap(ctx context.Context) error {
	status, _, err := i.request(ctx, http.MethodHead, "/"+i.physicalIndex, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		if err := i.validateMapping(ctx, i.physicalIndex); err != nil {
			return err
		}
		return i.validateAliases(ctx, i.physicalIndex)
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("inspect Elasticsearch index: unexpected status %d", status)
	}
	body, err := json.Marshal(map[string]any{
		"settings": map[string]any{"number_of_shards": i.shards, "number_of_replicas": i.replicas},
		"mappings": messageSearchMapping,
		"aliases": map[string]any{
			i.readAlias:  map[string]any{},
			i.writeAlias: map[string]any{"is_write_index": true},
		},
	})
	if err != nil {
		return fmt.Errorf("encode Elasticsearch index schema: %w", err)
	}
	status, response, err := i.request(ctx, http.MethodPut, "/"+i.physicalIndex, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("create Elasticsearch index", status, response)
	}
	return nil
}

func (i *Index) validateMapping(ctx context.Context, physicalIndex string) error {
	status, response, err := i.request(ctx, http.MethodGet, "/"+physicalIndex+"/_mapping", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("validate Elasticsearch mapping", status, response)
	}
	var mappings map[string]struct {
		Mappings struct {
			Dynamic    string `json:"dynamic"`
			Properties map[string]struct {
				Type  string `json:"type"`
				Index *bool  `json:"index,omitempty"`
			} `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(response, &mappings); err != nil {
		return fmt.Errorf("decode Elasticsearch mapping: %w", err)
	}
	mapping, ok := mappings[physicalIndex]
	if !ok || mapping.Mappings.Dynamic != "strict" {
		return fmt.Errorf("Elasticsearch index %s does not use the strict v1 mapping", physicalIndex)
	}
	expectedTypes := map[string]string{
		"message_uuid": "keyword", "conversation_key": "keyword", "message_seq": "long", "revision": "long",
		"sender_uuid": "keyword", "message_type": "byte", "content": "text", "sent_at": "date", "payload_hash": "keyword",
	}
	if len(mapping.Mappings.Properties) != len(expectedTypes) {
		return fmt.Errorf("Elasticsearch index %s has unexpected v1 mapping fields", physicalIndex)
	}
	for field, expectedType := range expectedTypes {
		property, ok := mapping.Mappings.Properties[field]
		if !ok || property.Type != expectedType {
			return fmt.Errorf("Elasticsearch v1 mapping field %s must use type %s", field, expectedType)
		}
		if field == "payload_hash" && (property.Index == nil || *property.Index) {
			return errors.New("Elasticsearch v1 mapping payload_hash must not be indexed")
		}
	}
	return nil
}

func (i *Index) SwitchAliases(ctx context.Context, fromIndex, toIndex string) error {
	if !indexPrefixPattern.MatchString(fromIndex) || !indexPrefixPattern.MatchString(toIndex) || fromIndex == toIndex {
		return errors.New("Elasticsearch alias switch indices are invalid")
	}
	if err := i.validateMapping(ctx, toIndex); err != nil {
		return fmt.Errorf("validate Elasticsearch alias target: %w", err)
	}
	actions := []any{
		map[string]any{"remove": map[string]any{"index": fromIndex, "alias": i.readAlias}},
		map[string]any{"remove": map[string]any{"index": fromIndex, "alias": i.writeAlias}},
		map[string]any{"add": map[string]any{"index": toIndex, "alias": i.readAlias}},
		map[string]any{"add": map[string]any{"index": toIndex, "alias": i.writeAlias, "is_write_index": true}},
	}
	body, err := json.Marshal(map[string]any{"actions": actions})
	if err != nil {
		return fmt.Errorf("encode Elasticsearch alias switch: %w", err)
	}
	status, response, err := i.request(ctx, http.MethodPost, "/_aliases", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("switch Elasticsearch aliases", status, response)
	}
	return nil
}

func (i *Index) Upsert(document *model.MessageSearchDocument) error {
	source, revision, err := normalizeDocument(document)
	if err != nil {
		return err
	}
	body, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("encode Elasticsearch search document: %w", err)
	}
	path := fmt.Sprintf("/%s/_doc/%s?require_alias=true&version=%d&version_type=external", i.writeAlias, url.PathEscape(source.MessageUUID), revision)
	status, response, err := i.request(context.Background(), http.MethodPut, path, body)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	if status == http.StatusConflict {
		return i.classifyVersionConflict(context.Background(), source)
	}
	return responseError("upsert Elasticsearch search document", status, response)
}

func (i *Index) Delete(messageUUID string) error {
	messageUUID = strings.TrimSpace(messageUUID)
	if messageUUID == "" {
		return errors.New("message uuid is required")
	}
	path := fmt.Sprintf("/%s/_doc/%s?require_alias=true", i.writeAlias, url.PathEscape(messageUUID))
	status, response, err := i.request(context.Background(), http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusNotFound {
		return nil
	}
	return responseError("delete Elasticsearch search document", status, response)
}

func (i *Index) Search(query model.MessageSearchQuery) ([]*model.MessageSearchDocument, error) {
	keys := uniqueSorted(query.ConversationKeys)
	if len(keys) == 0 {
		return nil, errors.New("search conversation scope is required")
	}
	text := strings.TrimSpace(query.Text)
	if text == "" {
		return nil, errors.New("search text is required")
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	body, err := json.Marshal(map[string]any{
		"size": limit,
		"sort": []any{map[string]any{"sent_at": "desc"}, map[string]any{"message_uuid": "desc"}},
		"query": map[string]any{"bool": map[string]any{
			"must":   []any{map[string]any{"match": map[string]any{"content": map[string]any{"query": text}}}},
			"filter": []any{map[string]any{"terms": map[string]any{"conversation_key": keys}}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Elasticsearch search query: %w", err)
	}
	status, response, err := i.request(context.Background(), http.MethodPost, "/"+i.readAlias+"/_search", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, responseError("search Elasticsearch messages", status, response)
	}
	var result struct {
		Hits struct {
			Hits []struct {
				Source searchDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode Elasticsearch search response: %w", err)
	}
	documents := make([]*model.MessageSearchDocument, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		document, err := modelDocument(hit.Source)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func (i *Index) classifyVersionConflict(ctx context.Context, candidate searchDocument) error {
	path := fmt.Sprintf("/%s/_doc/%s", i.writeAlias, url.PathEscape(candidate.MessageUUID))
	status, response, err := i.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("read Elasticsearch version conflict", status, response)
	}
	var current struct {
		Source searchDocument `json:"_source"`
	}
	if err := json.Unmarshal(response, &current); err != nil {
		return fmt.Errorf("decode Elasticsearch version conflict: %w", err)
	}
	switch {
	case current.Source.Revision > candidate.Revision:
		return nil
	case current.Source.Revision == candidate.Revision && current.Source.PayloadHash == candidate.PayloadHash:
		return nil
	case current.Source.Revision == candidate.Revision:
		return fmt.Errorf("%w: message=%s revision=%d", ErrProjectionConflict, candidate.MessageUUID, candidate.Revision)
	default:
		return fmt.Errorf("Elasticsearch conflict state regressed: message=%s current=%d candidate=%d", candidate.MessageUUID, current.Source.Revision, candidate.Revision)
	}
}

func (i *Index) validateAliases(ctx context.Context, physicalIndex string) error {
	path := fmt.Sprintf("/%s/_alias/%s,%s", physicalIndex, i.readAlias, i.writeAlias)
	status, response, err := i.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("validate Elasticsearch aliases", status, response)
	}
	var aliases map[string]struct {
		Aliases map[string]struct {
			IsWriteIndex bool `json:"is_write_index"`
		} `json:"aliases"`
	}
	if err := json.Unmarshal(response, &aliases); err != nil {
		return fmt.Errorf("decode Elasticsearch aliases: %w", err)
	}
	index, ok := aliases[physicalIndex]
	if !ok {
		return fmt.Errorf("Elasticsearch alias response is missing index %s", physicalIndex)
	}
	if _, ok := index.Aliases[i.readAlias]; !ok {
		return fmt.Errorf("Elasticsearch read alias %s is missing", i.readAlias)
	}
	write, ok := index.Aliases[i.writeAlias]
	if !ok || !write.IsWriteIndex {
		return fmt.Errorf("Elasticsearch write alias %s is not active", i.writeAlias)
	}
	return nil
}

func (i *Index) request(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	target := *i.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + strings.SplitN(path, "?", 2)[0]
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 {
		target.RawQuery = parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("create Elasticsearch request: %w", err)
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := i.client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("execute Elasticsearch request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("read Elasticsearch response: %w", err)
	}
	return response.StatusCode, responseBody, nil
}

func normalizeDocument(document *model.MessageSearchDocument) (searchDocument, uint64, error) {
	if document == nil {
		return searchDocument{}, 0, errors.New("search document is required")
	}
	revision := document.Revision
	if revision == 0 {
		revision = 1
	}
	source := searchDocument{
		MessageUUID: strings.TrimSpace(document.MessageUUID), ConversationKey: strings.TrimSpace(document.ConversationKey),
		MessageSeq: document.MessageSeq, Revision: revision, SenderUUID: strings.TrimSpace(document.SenderUUID),
		MessageType: document.MessageType, Content: document.Content, SentAt: document.SentAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if source.MessageUUID == "" || source.ConversationKey == "" || source.MessageSeq == 0 || source.SenderUUID == "" || document.SentAt.IsZero() {
		return searchDocument{}, 0, errors.New("search document identity is required")
	}
	if source.MessageSeq > math.MaxInt64 || revision > math.MaxInt64 {
		return searchDocument{}, 0, errors.New("search document sequence and revision must fit Elasticsearch long")
	}
	payload, err := json.Marshal(source)
	if err != nil {
		return searchDocument{}, 0, fmt.Errorf("hash Elasticsearch search document: %w", err)
	}
	sum := sha256.Sum256(payload)
	source.PayloadHash = hex.EncodeToString(sum[:])
	return source, revision, nil
}

func modelDocument(source searchDocument) (*model.MessageSearchDocument, error) {
	sentAt, err := time.Parse(time.RFC3339Nano, source.SentAt)
	if err != nil {
		return nil, fmt.Errorf("decode Elasticsearch message %s sent_at: %w", source.MessageUUID, err)
	}
	return &model.MessageSearchDocument{
		MessageUUID: source.MessageUUID, ConversationKey: source.ConversationKey, MessageSeq: source.MessageSeq,
		Revision: source.Revision, SenderUUID: source.SenderUUID, MessageType: source.MessageType, Content: source.Content,
		SentAt: sentAt,
	}, nil
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func responseError(operation string, status int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	if len(detail) > 512 {
		detail = detail[:512]
	}
	return fmt.Errorf("%s: status=%d body=%s", operation, status, detail)
}
