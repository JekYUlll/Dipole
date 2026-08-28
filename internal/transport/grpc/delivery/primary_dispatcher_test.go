package deliverygrpc

import (
	"context"
	"testing"
	"time"

	deliverycontract "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
)

type scriptedPrimarySink struct {
	statuses map[string][]ConnectionDeliveryStatus
	calls    [][]string
}

func (s *scriptedPrimarySink) Enqueue(_ context.Context, request ConnectionDeliveryRequest) ([]ConnectionDeliveryOutcome, error) {
	s.calls = append(s.calls, append([]string(nil), request.ConnectionIDs...))
	results := make([]ConnectionDeliveryOutcome, 0, len(request.ConnectionIDs))
	for _, connectionID := range request.ConnectionIDs {
		sequence := s.statuses[connectionID]
		status := ConnectionDeliveryStatusOffline
		if len(sequence) > 0 {
			status = sequence[0]
			s.statuses[connectionID] = sequence[1:]
		}
		result := ConnectionDeliveryOutcome{ConnectionID: connectionID, Status: status}
		if status == ConnectionDeliveryStatusBackpressured {
			result.QueueDepth = 1
			result.QueueCapacity = 1
		}
		results = append(results, result)
	}
	return results, nil
}

func TestPrimaryDispatcherRetriesOnlyIncompleteConnections(t *testing.T) {
	sink := &scriptedPrimarySink{statuses: map[string][]ConnectionDeliveryStatus{
		"C1": {ConnectionDeliveryStatusEnqueued},
		"C2": {ConnectionDeliveryStatusBackpressured, ConnectionDeliveryStatusEnqueued},
	}}
	dispatcher, err := NewPrimaryDispatcher("gateway-1", 64, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	batch := validNodeBatch("NB-primary", "gateway-1")

	first, err := dispatcher.DeliverNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := deliverycontract.ValidateAck(first); err != nil {
		t.Fatalf("validate first ack: %v", err)
	}
	if first.Status != deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_PARTIAL ||
		first.Results[0].Status != deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_BACKPRESSURED ||
		first.Results[0].AcceptedConnections != 1 {
		t.Fatalf("unexpected partial ack: %+v", first)
	}

	second, err := dispatcher.DeliverNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := deliverycontract.ValidateAck(second); err != nil {
		t.Fatalf("validate second ack: %v", err)
	}
	if second.Status != deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED ||
		second.Results[0].Status != deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED ||
		second.Results[0].AcceptedConnections != 2 {
		t.Fatalf("unexpected completed ack: %+v", second)
	}

	third, err := dispatcher.DeliverNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if third.Results[0].AcceptedConnections != 2 || len(sink.calls) != 2 ||
		len(sink.calls[0]) != 2 || len(sink.calls[1]) != 1 || sink.calls[1][0] != "C2" {
		t.Fatalf("unsafe replay calls=%v ack=%+v", sink.calls, third)
	}
}

func TestPrimaryDispatcherRejectsStableIdentityDrift(t *testing.T) {
	sink := &scriptedPrimarySink{statuses: map[string][]ConnectionDeliveryStatus{
		"C1": {ConnectionDeliveryStatusEnqueued}, "C2": {ConnectionDeliveryStatusEnqueued},
	}}
	dispatcher, err := NewPrimaryDispatcher("gateway-1", 64, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	batch := validNodeBatch("NB-drift", "gateway-1")
	if _, err := dispatcher.DeliverNodeBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	batch.Items[0].PayloadJson = []byte(`{"message_id":"changed"}`)

	ack, err := dispatcher.DeliverNodeBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Status != deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_REJECTED ||
		ack.Results[0].Status != deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_REJECTED || len(sink.calls) != 1 {
		t.Fatalf("identity drift was not rejected: calls=%v ack=%+v", sink.calls, ack)
	}
}
