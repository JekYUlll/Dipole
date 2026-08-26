package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
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

func (r *SearchIndexRepository) Upsert(document *model.MessageSearchDocument) error {
	if document == nil {
		return errors.New("search document is required")
	}
	if strings.TrimSpace(document.MessageUUID) == "" || strings.TrimSpace(document.ConversationKey) == "" {
		return errors.New("search document identity is required")
	}
	return r.queries.UpsertMessageSearchDocument(context.Background(), generated.UpsertMessageSearchDocumentParams{
		MessageUuid: strings.TrimSpace(document.MessageUUID), ConversationKey: strings.TrimSpace(document.ConversationKey),
		MessageSeq: document.MessageSeq, SenderUuid: strings.TrimSpace(document.SenderUUID), MessageType: document.MessageType,
		Content: document.Content, SentAt: document.SentAt,
	})
}

func (r *SearchIndexRepository) Delete(messageUUID string) error {
	messageUUID = strings.TrimSpace(messageUUID)
	if messageUUID == "" {
		return errors.New("message uuid is required")
	}
	return r.queries.DeleteMessageSearchDocument(context.Background(), messageUUID)
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
	rows, err := r.queries.SearchMessageDocuments(context.Background(), generated.SearchMessageDocumentsParams{
		ConversationKeys: conversationKeys, SearchText: escapeLikeLiteral(text), Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search messages with sqlc: %w", err)
	}
	results := make([]*model.MessageSearchDocument, 0, len(rows))
	for _, row := range rows {
		results = append(results, &model.MessageSearchDocument{
			MessageUUID: row.MessageUuid, ConversationKey: row.ConversationKey, MessageSeq: row.MessageSeq,
			SenderUUID: row.SenderUuid, MessageType: row.MessageType, Content: row.Content, SentAt: row.SentAt,
		})
	}
	return results, nil
}

func escapeLikeLiteral(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
