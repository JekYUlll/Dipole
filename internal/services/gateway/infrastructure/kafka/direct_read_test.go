package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type directReadSender struct {
	user  string
	type_ string
	data  any
	ids   correlation.IDs
}

func (s *directReadSender) SendEventToUser(userUUID, eventType string, data any) int {
	s.user, s.type_, s.data = userUUID, eventType, data
	return 1
}

func (s *directReadSender) SendEventToUserContext(ctx context.Context, userUUID, eventType string, data any) int {
	s.ids = correlation.FromContext(ctx)
	return s.SendEventToUser(userUUID, eventType, data)
}

func TestNewDirectReadHandlerPreservesReceiptAndContext(t *testing.T) {
	sender := &directReadSender{}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1", EventID: "E1"})
	readAt := time.Now().UTC().Truncate(time.Millisecond)
	payload, err := json.Marshal(coreconversation.ConversationReadReceipt{
		ReaderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		ConversationKey: "direct:U1:U2", LastReadMessageUUID: "M9", LastReadSeq: 9, ReadAt: readAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "conversation.direct.read", Payload: payload}}

	if err := NewDirectReadHandler(sender)(ctx, event); err != nil {
		t.Fatalf("handle direct read: %v", err)
	}
	if sender.user != "U2" || sender.type_ != wsTransport.TypeChatRead || sender.ids != (correlation.IDs{RequestID: "R1", TraceID: "T1", EventID: "E1"}) {
		t.Fatalf("unexpected delivery: %+v", sender)
	}
	data, ok := sender.data.(wsTransport.ChatReadData)
	if !ok || data.LastReadMessageUUID != "M9" || data.LastReadSeq != 9 || data.ConversationKey != "direct:U1:U2" {
		t.Fatalf("unexpected read data: %#v", sender.data)
	}
}

func TestNewDirectReadHandlerRejectsMalformedEvent(t *testing.T) {
	sender := &directReadSender{}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "conversation.direct.read", Payload: []byte(`{"target_uuid":`)}}
	if err := NewDirectReadHandler(sender)(context.Background(), event); err == nil {
		t.Fatal("malformed direct read must retain the Kafka record")
	} else if errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected context error: %v", err)
	}
}
