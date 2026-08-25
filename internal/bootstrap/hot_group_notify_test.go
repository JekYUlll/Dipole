package bootstrap

import (
	"sync"
	"testing"
	"time"

	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type stubHotGroupHub struct {
	mu     sync.Mutex
	events map[string]wsTransport.GroupMessageNotifyData
	counts map[string]int
}

func (s *stubHotGroupHub) SendEventToUser(userUUID, eventType string, data any) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.events == nil {
		s.events = make(map[string]wsTransport.GroupMessageNotifyData)
		s.counts = make(map[string]int)
	}
	if eventType == wsTransport.TypeGroupMessageNotify {
		s.events[userUUID] = data.(wsTransport.GroupMessageNotifyData)
		s.counts[userUUID]++
	}
	return 1
}

func (s *stubHotGroupHub) DisconnectConnections(userUUID string, connectionIDs []string, reason string) int {
	return 0
}

func (s *stubHotGroupHub) DisconnectAllConnections(userUUID string, reason string) int {
	return 0
}

func TestHotGroupNotifyAggregatorCoalescesWindow(t *testing.T) {
	t.Parallel()

	hub := &stubHotGroupHub{}
	aggregator := newHotGroupNotifyAggregator(hub, 20*time.Millisecond)

	aggregator.Enqueue("G100", wsTransport.GroupMessageNotifyData{
		GroupUUID:       "G100",
		LatestMessageID: "M1",
		Preview:         "first",
	}, []string{"U1", "U2"})
	aggregator.Enqueue("G100", wsTransport.GroupMessageNotifyData{
		GroupUUID:       "G100",
		LatestMessageID: "M2",
		Preview:         "second",
	}, []string{"U2", "U3"})

	time.Sleep(60 * time.Millisecond)

	hub.mu.Lock()
	defer hub.mu.Unlock()

	if len(hub.events) != 3 {
		t.Fatalf("expected 3 recipients after coalesced flush, got %d", len(hub.events))
	}
	for _, uuid := range []string{"U1", "U2", "U3"} {
		event, ok := hub.events[uuid]
		if !ok {
			t.Fatalf("expected recipient %s to receive coalesced notify", uuid)
		}
		if event.LatestMessageID != "M2" {
			t.Fatalf("expected latest message id M2 for %s, got %s", uuid, event.LatestMessageID)
		}
		if hub.counts[uuid] != 1 {
			t.Fatalf("expected recipient %s to receive one notify, got %d", uuid, hub.counts[uuid])
		}
	}
}
