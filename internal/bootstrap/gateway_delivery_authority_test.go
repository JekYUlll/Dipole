package bootstrap

import (
	"context"
	"testing"

	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

func TestGatewayMessageHandlersCheckpointOnlyInCPPMode(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, group, err := gatewayMessageDeliveryHandlers(
		realtimeDelivery.AuthorityCPP,
		sender,
		fixedGroupHeat{},
		newHotGroupNotifyAggregator(sender, 0),
		wsTransport.TimelineNotifyOff,
	)
	if err != nil {
		t.Fatalf("compose C++ authority handlers: %v", err)
	}
	if err := direct(context.Background(), directCreatedEvent(t, 42)); err != nil {
		t.Fatalf("checkpoint direct event: %v", err)
	}
	if err := group(context.Background(), groupCreatedEvent(t)); err != nil {
		t.Fatalf("checkpoint group event: %v", err)
	}
	if got := len(sender.snapshot()); got != 0 {
		t.Fatalf("C++ authority performed %d Go client writes", got)
	}
}

func TestGatewayCheckpointHandlerRejectsMalformedMessage(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, _, err := gatewayMessageDeliveryHandlers(
		realtimeDelivery.AuthorityCPP,
		sender,
		fixedGroupHeat{},
		newHotGroupNotifyAggregator(sender, 0),
		wsTransport.TimelineNotifyOff,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := direct(context.Background(), directCreatedEvent(t, 42)); err != nil {
		t.Fatalf("valid checkpoint failed: %v", err)
	}
	malformed := directCreatedEvent(t, 42)
	malformed.Envelope.Payload = []byte(`{"message_id":`)
	if err := direct(context.Background(), malformed); err == nil {
		t.Fatal("malformed checkpoint must retain the Kafka record")
	}
}

func TestGatewayMessageHandlersKeepGoWritesInShadowMode(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, _, err := gatewayMessageDeliveryHandlers(
		realtimeDelivery.AuthorityShadow,
		sender,
		fixedGroupHeat{},
		newHotGroupNotifyAggregator(sender, 0),
		wsTransport.TimelineNotifyOff,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := direct(context.Background(), directCreatedEvent(t, 42)); err != nil {
		t.Fatalf("deliver direct event: %v", err)
	}
	events := sender.snapshot()
	if len(events) != 1 || events[0].eventType != wsTransport.TypeChatMessage {
		t.Fatalf("shadow mode Go writes = %+v", events)
	}
}
