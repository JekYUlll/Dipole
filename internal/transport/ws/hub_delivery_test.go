package ws

import (
	"context"
	"testing"
)

func TestHubEnqueueEventToConnectionsTargetsExactConnections(t *testing.T) {
	hub := NewHub()
	first := deliveryTestClient("U1", "C1", 2)
	second := deliveryTestClient("U1", "C2", 2)
	hub.Register(first)
	hub.Register(second)

	results, err := hub.EnqueueEventToConnectionsContext(
		context.Background(), "U1", []string{"C1", "missing"}, "message.direct.created", map[string]string{"message_id": "M1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ConnectionID != "C1" || results[0].Status != ConnectionEnqueueStatusEnqueued ||
		results[1].ConnectionID != "missing" || results[1].Status != ConnectionEnqueueStatusOffline {
		t.Fatalf("unexpected targeted results: %+v", results)
	}
	if len(first.send) != 1 {
		t.Fatalf("target queue depth = %d, want 1", len(first.send))
	}
	if len(second.send) != 0 {
		t.Fatalf("untargeted connection received %d messages", len(second.send))
	}
}

func TestHubEnqueueEventToConnectionsReportsQueuePressure(t *testing.T) {
	hub := NewHub()
	client := deliveryTestClient("U1", "C1", 1)
	hub.Register(client)
	client.send <- []byte("occupied")

	results, err := hub.EnqueueEventToConnectionsContext(
		context.Background(), "U1", []string{"C1"}, "message.direct.created", map[string]string{"message_id": "M1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != ConnectionEnqueueStatusBackpressured ||
		results[0].QueueDepth != 1 || results[0].QueueCapacity != 1 {
		t.Fatalf("unexpected pressure result: %+v", results)
	}
}

func TestHubEnqueueEventToConnectionsRejectsDuplicateTargets(t *testing.T) {
	hub := NewHub()
	hub.Register(deliveryTestClient("U1", "C1", 1))
	if _, err := hub.EnqueueEventToConnectionsContext(
		context.Background(), "U1", []string{"C1", "C1"}, "message.direct.created", map[string]string{"message_id": "M1"},
	); err == nil {
		t.Fatal("duplicate connection target must fail before enqueue")
	}
}

func deliveryTestClient(userID, connectionID string, capacity int) *Client {
	return &Client{
		sessionUser:  &SessionUser{UUID: userID},
		connectionID: connectionID,
		send:         make(chan []byte, capacity),
	}
}
