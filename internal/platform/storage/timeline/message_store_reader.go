package timeline

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

// MessageStoreReader adapts the current SQLC-backed MessageStore to the same
// range contract as Cassandra, enabling identical routing and benchmark inputs.
type MessageStoreReader struct{ store application.MessageStore }

var _ application.ConversationTimelineReader = (*MessageStoreReader)(nil)

func NewMessageStoreReader(store application.MessageStore) (*MessageStoreReader, error) {
	if store == nil {
		return nil, fmt.Errorf("message timeline reader requires a MessageStore")
	}
	return &MessageStoreReader{store: store}, nil
}

func (r *MessageStoreReader) ListConversationRange(_ context.Context, conversationKey string, firstSeq, lastSeq uint64) ([]*model.Message, error) {
	if firstSeq == 0 || lastSeq == 0 || firstSeq > lastSeq {
		return nil, fmt.Errorf("message timeline range is invalid")
	}
	return r.store.ListByConversationSeqAfter(conversationKey, firstSeq-1, int(lastSeq-firstSeq+1))
}
