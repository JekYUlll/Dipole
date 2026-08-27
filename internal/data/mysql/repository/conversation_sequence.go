package repository

import (
	"context"
	"strings"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type ConversationSequenceRepository struct {
	queries *generated.Queries
}

func NewConversationSequenceRepository(queries *generated.Queries) *ConversationSequenceRepository {
	return &ConversationSequenceRepository{queries: queries}
}

func (r *ConversationSequenceRepository) LatestConversationSequence(conversationKey string) (uint64, error) {
	return r.queries.GetConversationSequence(context.Background(), strings.TrimSpace(conversationKey))
}
