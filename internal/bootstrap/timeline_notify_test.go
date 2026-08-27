package bootstrap

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type recordedWSEvent struct {
	userUUID  string
	eventType string
	data      any
	ids       correlation.IDs
}

type recordingWSEventSender struct {
	mu     sync.Mutex
	events []recordedWSEvent
}

func (*recordingWSEventSender) DisconnectConnections(string, []string, string) int { return 0 }
func (*recordingWSEventSender) DisconnectAllConnections(string, string) int        { return 0 }

func (s *recordingWSEventSender) SendEventToUser(userUUID, eventType string, data any) int {
	return s.SendEventToUserContext(context.Background(), userUUID, eventType, data)
}

func (s *recordingWSEventSender) SendEventToUserContext(ctx context.Context, userUUID, eventType string, data any) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, recordedWSEvent{userUUID: userUUID, eventType: eventType, data: data, ids: correlation.FromContext(ctx)})
	return 1
}

func (s *recordingWSEventSender) snapshot() []recordedWSEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedWSEvent(nil), s.events...)
}

func TestDeliverDirectMessageTimelineNotificationModes(t *testing.T) {
	for _, test := range []struct {
		name      string
		mode      string
		wantTypes []string
	}{
		{name: "off", mode: wsTransport.TimelineNotifyOff, wantTypes: []string{wsTransport.TypeChatMessage}},
		{name: "shadow", mode: wsTransport.TimelineNotifyShadow, wantTypes: []string{wsTransport.TypeChatMessage, wsTransport.TypeSyncItemNotifyV1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			sender := &recordingWSEventSender{}
			event := directCreatedEvent(t, 42)
			ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R42", TraceID: "T42", EventID: "E42"})
			if err := deliverDirectMessageHandler(sender, test.mode)(ctx, event); err != nil {
				t.Fatalf("deliver direct event: %v", err)
			}
			if len(sender.events) != len(test.wantTypes) {
				t.Fatalf("event count=%d want=%d: %+v", len(sender.events), len(test.wantTypes), sender.events)
			}
			for index, wantType := range test.wantTypes {
				if sender.events[index].eventType != wantType || sender.events[index].userUUID != "U2" {
					t.Fatalf("event[%d]=%+v want type=%s user=U2", index, sender.events[index], wantType)
				}
				if sender.events[index].ids != (correlation.IDs{RequestID: "R42", TraceID: "T42", EventID: "E42"}) {
					t.Fatalf("event[%d] correlation=%+v", index, sender.events[index].ids)
				}
			}
			if test.mode == wsTransport.TimelineNotifyShadow {
				notify := sender.events[1].data.(wsTransport.SyncItemNotifyData)
				if notify.SchemaVersion != "v1" || notify.EventID != "E42" || notify.MessageUUID != "M42" || notify.ConversationKey != "direct:U1:U2" || notify.MessageSeq != 42 {
					t.Fatalf("unexpected notify: %+v", notify)
				}
				if notify.TargetType != model.MessageTargetDirect || notify.TargetUUID != "U2" {
					t.Fatalf("unexpected target locator: %+v", notify)
				}
			}
		})
	}
}

type fixedGroupHeat struct{ hot bool }

func (h fixedGroupHeat) Status(string, int) (platformHotGroup.Status, error) {
	return platformHotGroup.Status{IsHot: h.hot, RecentMessageCount: 7}, nil
}

func TestDeliverGroupMessageKeepsHotGroupAggregation(t *testing.T) {
	for _, test := range []struct {
		name      string
		hot       bool
		wantTypes []string
	}{
		{name: "normal group", wantTypes: []string{wsTransport.TypeChatMessage, wsTransport.TypeSyncItemNotifyV1}},
		{name: "hot group", hot: true, wantTypes: []string{wsTransport.TypeGroupMessageNotify, wsTransport.TypeGroupMessageNotify}},
	} {
		t.Run(test.name, func(t *testing.T) {
			sender := &recordingWSEventSender{}
			aggregator := newHotGroupNotifyAggregator(sender, time.Millisecond)
			if err := deliverGroupMessageHandler(sender, fixedGroupHeat{hot: test.hot}, aggregator, wsTransport.TimelineNotifyShadow)(context.Background(), groupCreatedEvent(t)); err != nil {
				t.Fatalf("deliver group event: %v", err)
			}
			if test.hot {
				deadline := time.Now().Add(time.Second)
				for len(sender.snapshot()) < len(test.wantTypes) && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
			}
			events := sender.snapshot()
			if len(events) != len(test.wantTypes) {
				t.Fatalf("event count=%d want=%d: %+v", len(events), len(test.wantTypes), events)
			}
			users := make(map[string]bool)
			for index, wantType := range test.wantTypes {
				if events[index].eventType != wantType {
					t.Fatalf("event[%d]=%+v want type=%s", index, events[index], wantType)
				}
				users[events[index].userUUID] = true
			}
			if test.hot {
				if !users["U1"] || !users["U2"] {
					t.Fatalf("hot-group recipients=%+v", users)
				}
			} else if !users["U2"] {
				t.Fatalf("normal-group recipients=%+v", users)
			}
		})
	}
}

func TestDeliverDirectMessageSkipsTimelineNotificationWithoutSequence(t *testing.T) {
	sender := &recordingWSEventSender{}
	if err := deliverDirectMessageHandler(sender, wsTransport.TimelineNotifyShadow)(context.Background(), directCreatedEvent(t, 0)); err != nil {
		t.Fatalf("deliver legacy direct event: %v", err)
	}
	if len(sender.events) != 1 || sender.events[0].eventType != wsTransport.TypeChatMessage {
		t.Fatalf("legacy event must keep full-message-only delivery: %+v", sender.events)
	}
}

func directCreatedEvent(t *testing.T, sequence uint64) platformKafka.Event {
	t.Helper()
	payload, err := json.Marshal(service.MessageEventPayload{
		MessageID: "M42", ConversationKey: "direct:U1:U2", MessageSeq: sequence,
		SenderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "secret body", SentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventID: "E42", EventType: "message.direct.created", Version: "v1", Payload: payload,
	}}
}

func groupCreatedEvent(t *testing.T) platformKafka.Event {
	t.Helper()
	payload, err := json.Marshal(service.MessageEventPayload{
		MessageID: "MG42", ConversationKey: "group:G1", MessageSeq: 42,
		SenderUUID: "U1", TargetUUID: "G1", TargetType: model.MessageTargetGroup,
		MessageType: model.MessageTypeText, Content: "group body", SentAt: time.Now().UTC(),
		RecipientUUIDs: []string{"U1", "U2"},
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventID: "EG42", EventType: "message.group.created", Version: "v1", Payload: payload,
	}}
}
