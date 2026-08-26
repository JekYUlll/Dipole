package elasticsearch

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

const mappingVersion = "v1"

var (
	//go:embed schema/message_search_v1.json
	messageSearchMapping json.RawMessage

	ErrProjectionConflict = model.ErrMessageSearchMutationConflict
	indexPrefixPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

type Config struct {
	Address     string
	IndexPrefix string
	Shards      int
	Replicas    int
	Username    string
	Password    string
	APIKey      string
	HTTPClient  *http.Client
}

type Index struct {
	baseURL       *url.URL
	client        *http.Client
	indexPrefix   string
	physicalIndex string
	readAlias     string
	writeAlias    string
	shards        int
	replicas      int
	username      string
	password      string
	apiKey        string
}

var _ application.SearchIndex = (*Index)(nil)

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
	username := strings.TrimSpace(config.Username)
	password := strings.TrimSpace(config.Password)
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey != "" && (username != "" || password != "") {
		return nil, errors.New("Elasticsearch API key and basic authentication are mutually exclusive")
	}
	if (username == "") != (password == "") {
		return nil, errors.New("Elasticsearch username and password must be configured together")
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Index{
		baseURL: baseURL, client: client, shards: config.Shards, replicas: config.Replicas,
		username: username, password: password, apiKey: apiKey,
		indexPrefix:   prefix,
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
	return i.createPhysicalIndex(ctx, i.physicalIndex, map[string]any{
		i.readAlias:  map[string]any{},
		i.writeAlias: map[string]any{"is_write_index": true},
	})
}

type PhysicalTarget struct {
	index *Index
	name  string
}

func (i *Index) CreatePhysicalTarget(ctx context.Context, name string) (*PhysicalTarget, error) {
	if err := i.validatePhysicalIndexName(name); err != nil {
		return nil, err
	}
	status, _, err := i.request(ctx, http.MethodHead, "/"+name, nil)
	if err != nil {
		return nil, err
	}
	switch status {
	case http.StatusOK:
		if err := i.validateMapping(ctx, name); err != nil {
			return nil, err
		}
	case http.StatusNotFound:
		if err := i.createPhysicalIndex(ctx, name, nil); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("inspect Elasticsearch physical target: unexpected status %d", status)
	}
	return &PhysicalTarget{index: i, name: name}, nil
}

func (i *Index) PhysicalTarget(ctx context.Context, name string) (*PhysicalTarget, error) {
	if err := i.validatePhysicalIndexName(name); err != nil {
		return nil, err
	}
	if err := i.validateMapping(ctx, name); err != nil {
		return nil, err
	}
	return &PhysicalTarget{index: i, name: name}, nil
}

func (i *Index) createPhysicalIndex(ctx context.Context, name string, aliases map[string]any) error {
	bodyValue := map[string]any{
		"settings": map[string]any{"number_of_shards": i.shards, "number_of_replicas": i.replicas},
		"mappings": messageSearchMapping,
	}
	if aliases != nil {
		bodyValue["aliases"] = aliases
	}
	body, err := json.Marshal(bodyValue)
	if err != nil {
		return fmt.Errorf("encode Elasticsearch index schema: %w", err)
	}
	status, response, err := i.request(ctx, http.MethodPut, "/"+name, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("create Elasticsearch index", status, response)
	}
	return nil
}

func (i *Index) validatePhysicalIndexName(name string) error {
	name = strings.TrimSpace(name)
	if !indexPrefixPattern.MatchString(name) || !strings.HasPrefix(name, i.indexPrefix+"-messages-") {
		return errors.New("Elasticsearch physical index name is invalid")
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
		"sender_uuid": "keyword", "message_type": "byte", "content": "text", "sent_at": "date", "searchable": "boolean", "payload_hash": "keyword",
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

func (i *Index) Apply(mutation *model.MessageSearchMutation) error {
	return i.apply(context.Background(), i.writeAlias, true, mutation)
}

func (i *Index) apply(ctx context.Context, target string, requireAlias bool, mutation *model.MessageSearchMutation) error {
	source, err := mutation.State()
	if err != nil {
		return err
	}
	body, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("encode Elasticsearch search document: %w", err)
	}
	query := fmt.Sprintf("version=%d&version_type=external", source.Revision)
	if requireAlias {
		query = "require_alias=true&" + query
	}
	path := fmt.Sprintf("/%s/_doc/%s?%s", target, url.PathEscape(source.MessageUUID), query)
	status, response, err := i.request(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	if status == http.StatusConflict {
		return i.classifyVersionConflict(ctx, target, source)
	}
	return responseError("upsert Elasticsearch search document", status, response)
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
			"must": []any{map[string]any{"match": map[string]any{"content": map[string]any{"query": text}}}},
			"filter": []any{
				map[string]any{"terms": map[string]any{"conversation_key": keys}},
				map[string]any{"term": map[string]any{"searchable": true}},
			},
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
				Source model.MessageSearchState `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode Elasticsearch search response: %w", err)
	}
	documents := make([]*model.MessageSearchDocument, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		if hit.Source.SentAt == nil {
			return nil, fmt.Errorf("Elasticsearch searchable document %s is missing sent_at", hit.Source.MessageUUID)
		}
		documents = append(documents, &model.MessageSearchDocument{
			MessageUUID: hit.Source.MessageUUID, ConversationKey: hit.Source.ConversationKey, MessageSeq: hit.Source.MessageSeq,
			Revision: hit.Source.Revision, SenderUUID: hit.Source.SenderUUID, MessageType: hit.Source.MessageType,
			Content: hit.Source.Content, SentAt: *hit.Source.SentAt,
		})
	}
	return documents, nil
}

func (i *Index) classifyVersionConflict(ctx context.Context, target string, candidate model.MessageSearchState) error {
	path := fmt.Sprintf("/%s/_doc/%s", target, url.PathEscape(candidate.MessageUUID))
	status, response, err := i.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("read Elasticsearch version conflict", status, response)
	}
	var current struct {
		Source model.MessageSearchState `json:"_source"`
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

func (t *PhysicalTarget) Apply(mutation *model.MessageSearchMutation) error {
	return t.index.apply(context.Background(), t.name, false, mutation)
}

func (t *PhysicalTarget) Lookup(ctx context.Context, messageUUID string) (model.MessageSearchState, bool, error) {
	path := fmt.Sprintf("/%s/_doc/%s", t.name, url.PathEscape(strings.TrimSpace(messageUUID)))
	status, response, err := t.index.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return model.MessageSearchState{}, false, err
	}
	if status == http.StatusNotFound {
		return model.MessageSearchState{}, false, nil
	}
	if status != http.StatusOK {
		return model.MessageSearchState{}, false, responseError("lookup Elasticsearch search document", status, response)
	}
	var current struct {
		Source model.MessageSearchState `json:"_source"`
	}
	if err := json.Unmarshal(response, &current); err != nil {
		return model.MessageSearchState{}, false, fmt.Errorf("decode Elasticsearch search document: %w", err)
	}
	return current.Source, true, nil
}

func (t *PhysicalTarget) Count(ctx context.Context) (uint64, error) {
	status, response, err := t.index.request(ctx, http.MethodPost, "/"+t.name+"/_count", []byte(`{"query":{"match_all":{}}}`))
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, responseError("count Elasticsearch search documents", status, response)
	}
	var result struct {
		Count uint64 `json:"count"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return 0, fmt.Errorf("decode Elasticsearch search document count: %w", err)
	}
	return result.Count, nil
}

func (t *PhysicalTarget) Refresh(ctx context.Context) error {
	status, response, err := t.index.request(ctx, http.MethodPost, "/"+t.name+"/_refresh", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError("refresh Elasticsearch physical target", status, response)
	}
	return nil
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
	if i.apiKey != "" {
		request.Header.Set("Authorization", "ApiKey "+i.apiKey)
	} else if i.username != "" {
		request.SetBasicAuth(i.username, i.password)
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
