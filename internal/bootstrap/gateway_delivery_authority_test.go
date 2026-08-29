package bootstrap

import (
	"context"
	"errors"
	"testing"

	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	gatewaykafka "github.com/JekYUlll/Dipole/internal/services/gateway/infrastructure/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type authorityFenceStub struct {
	err   error
	calls int
	local realtimeDelivery.Authority
}

func (s *authorityFenceStub) Assert(_ context.Context, local realtimeDelivery.Authority) error {
	s.calls++
	s.local = local
	return s.err
}

type recoveringAuthorityFenceStub struct {
	calls int
}

func (s *recoveringAuthorityFenceStub) Assert(_ context.Context, _ realtimeDelivery.Authority) error {
	s.calls++
	if s.calls == 1 {
		return errors.New("temporarily frozen")
	}
	return nil
}

func TestGatewayMessageHandlersCheckpointOnlyInCPPMode(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, group, err := gatewaykafka.NewMessageDeliveryHandlers(
		realtimeDelivery.AuthorityCPP,
		sender,
		fixedGroupHeat{},
		gatewaykafka.NewNotifier(sender, 0),
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
	direct, _, err := gatewaykafka.NewMessageDeliveryHandlers(
		realtimeDelivery.AuthorityCPP,
		sender,
		fixedGroupHeat{},
		gatewaykafka.NewNotifier(sender, 0),
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
	direct, _, err := gatewaykafka.NewMessageDeliveryHandlers(
		realtimeDelivery.AuthorityShadow,
		sender,
		fixedGroupHeat{},
		gatewaykafka.NewNotifier(sender, 0),
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

func TestGatewayMessageHandlerChecksSharedFenceBeforeClientWrite(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, _, err := gatewaykafka.NewMessageDeliveryHandlers(
		realtimeDelivery.AuthorityGo,
		sender,
		fixedGroupHeat{},
		gatewaykafka.NewNotifier(sender, 0),
		wsTransport.TimelineNotifyOff,
	)
	if err != nil {
		t.Fatal(err)
	}
	fence := &authorityFenceStub{err: errors.New("frozen")}
	guarded := gatewaykafka.FenceMessageDeliveryHandler(realtimeDelivery.AuthorityGo, fence, direct)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := guarded(ctx, directCreatedEvent(t, 42)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled fence wait error = %v", err)
	}
	if fence.calls != 1 || fence.local != realtimeDelivery.AuthorityGo {
		t.Fatalf("fence calls/local = %d/%q", fence.calls, fence.local)
	}
	if got := len(sender.snapshot()); got != 0 {
		t.Fatalf("denied fence performed %d client writes", got)
	}

	fence.err = nil
	if err := guarded(context.Background(), directCreatedEvent(t, 42)); err != nil {
		t.Fatalf("authorized fence: %v", err)
	}
	if got := len(sender.snapshot()); got != 1 {
		t.Fatalf("authorized fence client writes = %d, want 1", got)
	}
}

func TestGatewayMessageHandlerContinuesSameRecordAfterFenceRecovery(t *testing.T) {
	sender := &recordingWSEventSender{}
	direct, _, err := gatewaykafka.NewMessageDeliveryHandlers(
		realtimeDelivery.AuthorityGo,
		sender,
		fixedGroupHeat{},
		gatewaykafka.NewNotifier(sender, 0),
		wsTransport.TimelineNotifyOff,
	)
	if err != nil {
		t.Fatal(err)
	}
	fence := &recoveringAuthorityFenceStub{}
	guarded := gatewaykafka.FenceMessageDeliveryHandler(realtimeDelivery.AuthorityGo, fence, direct)
	if err := guarded(context.Background(), directCreatedEvent(t, 42)); err != nil {
		t.Fatalf("recover fence on same record: %v", err)
	}
	if fence.calls != 2 {
		t.Fatalf("fence calls = %d, want 2", fence.calls)
	}
	if got := len(sender.snapshot()); got != 1 {
		t.Fatalf("client writes after recovery = %d, want 1", got)
	}
}
