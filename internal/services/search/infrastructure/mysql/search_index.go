package searchmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.SearchIndex = (*SearchIndexRepository)(nil)

type SearchIndexRepository struct{ queries *generated.Queries }

func NewSearchIndexRepository(queries *generated.Queries) (*SearchIndexRepository, error) {
	if queries == nil {
		return nil, errors.New("search index queries are required")
	}
	return &SearchIndexRepository{queries: queries}, nil
}

func (r *SearchIndexRepository) Apply(mutation *model.MessageSearchMutation) error {
	state, err := mutation.State()
	if err != nil {
		return err
	}
	params := generated.ApplyMessageSearchStateParams{
		MessageUuid: state.MessageUUID, Revision: state.Revision,
		Searchable: state.Searchable, PayloadHash: state.PayloadHash,
	}
	if state.Searchable {
		params.ConversationKey = sql.NullString{String: state.ConversationKey, Valid: true}
		params.MessageSeq = sql.NullInt64{Int64: int64(state.MessageSeq), Valid: true}
		params.SenderUuid = sql.NullString{String: state.SenderUUID, Valid: true}
		params.MessageType = sql.NullInt16{Int16: int16(state.MessageType), Valid: true}
		params.Content = sql.NullString{String: state.Content, Valid: true}
		params.SentAt = sql.NullTime{Time: *state.SentAt, Valid: true}
	}
	ctx := context.Background()
	if err := r.queries.ApplyMessageSearchState(ctx, params); err != nil {
		return fmt.Errorf("apply search mutation with sqlc: %w", err)
	}
	current, err := r.queries.GetMessageSearchState(ctx, state.MessageUUID)
	if err != nil {
		return fmt.Errorf("read applied search mutation with sqlc: %w", err)
	}
	switch {
	case current.Revision > state.Revision:
		return nil
	case current.Revision == state.Revision && current.PayloadHash == state.PayloadHash:
		return nil
	case current.Revision == state.Revision:
		return fmt.Errorf("%w: message=%s revision=%d", model.ErrMessageSearchMutationConflict, state.MessageUUID, state.Revision)
	default:
		return fmt.Errorf("MySQL search mutation state regressed: message=%s current=%d candidate=%d", state.MessageUUID, current.Revision, state.Revision)
	}
}

func (r *SearchIndexRepository) Search(query model.MessageSearchQuery) ([]*model.MessageSearchDocument, error) {
	conversationKeys := uniqueSortedSQLCUUIDs(query.ConversationKeys)
	if len(conversationKeys) == 0 {
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
	scopes := make([]sql.NullString, 0, len(conversationKeys))
	for _, conversationKey := range conversationKeys {
		scopes = append(scopes, sql.NullString{String: conversationKey, Valid: true})
	}
	rows, err := r.queries.SearchMessageDocuments(context.Background(), generated.SearchMessageDocumentsParams{
		ConversationKeys: scopes, SearchText: escapeLikeLiteral(text), Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search messages with sqlc: %w", err)
	}
	results := make([]*model.MessageSearchDocument, 0, len(rows))
	for _, row := range rows {
		if !row.ConversationKey.Valid || !row.MessageSeq.Valid || !row.SenderUuid.Valid || !row.MessageType.Valid || !row.Content.Valid || !row.SentAt.Valid {
			return nil, fmt.Errorf("search document %s is missing searchable fields", row.MessageUuid)
		}
		results = append(results, &model.MessageSearchDocument{
			MessageUUID: row.MessageUuid, ConversationKey: row.ConversationKey.String, MessageSeq: uint64(row.MessageSeq.Int64),
			Revision: row.Revision, SenderUUID: row.SenderUuid.String, MessageType: int8(row.MessageType.Int16),
			Content: row.Content.String, SentAt: row.SentAt.Time,
		})
	}
	return results, nil
}

func escapeLikeLiteral(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
