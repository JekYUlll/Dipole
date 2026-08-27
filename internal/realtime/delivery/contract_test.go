package delivery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGoldenBatchV1IsStableAndValid(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "api", "proto", "dipole", "delivery", "v1", "testdata", "delivery_batch.v1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var batch deliveryv1.DeliveryEnvelope
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, &batch); err != nil {
		t.Fatalf("decode golden batch: %v", err)
	}
	if err := ValidateEnvelope(&batch); err != nil {
		t.Fatalf("validate golden batch: %v", err)
	}
	if got, want := len(batch.Items), 2; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
}

func TestGoldenNodeBatchAndAckV1AreStableAndValid(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("..", "..", "..", "api", "proto", "dipole", "delivery", "v1", "testdata")
	nodeRaw, err := os.ReadFile(filepath.Join(testdata, "node_delivery_batch.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var batch deliveryv1.NodeDeliveryBatch
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(nodeRaw, &batch); err != nil {
		t.Fatalf("decode golden node batch: %v", err)
	}
	if err := ValidateNodeBatch(&batch); err != nil {
		t.Fatalf("validate golden node batch: %v", err)
	}

	ackRaw, err := os.ReadFile(filepath.Join(testdata, "delivery_ack.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ack deliveryv1.DeliveryAck
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(ackRaw, &ack); err != nil {
		t.Fatalf("decode golden ack: %v", err)
	}
	if err := ValidateAck(&ack); err != nil {
		t.Fatalf("validate golden ack: %v", err)
	}

	observationRaw, err := os.ReadFile(filepath.Join(testdata, "node_delivery_observation.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observation deliveryv1.NodeDeliveryObservation
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(observationRaw, &observation); err != nil {
		t.Fatalf("decode golden observation: %v", err)
	}
	if err := ValidateNodeObservation(&observation); err != nil {
		t.Fatalf("validate golden observation: %v", err)
	}
}

func TestValidateNodeBatchRequiresResolvedConnections(t *testing.T) {
	t.Parallel()

	batch := &deliveryv1.NodeDeliveryBatch{
		ContractVersion: ContractVersion, BatchId: "NB1", TargetNodeId: "gateway-1", SourceEventId: "E1",
		CreatedAt: timestamppb.New(time.Unix(1, 0).UTC()),
		Items: []*deliveryv1.NodeDeliveryItem{{
			DeliveryId: "D1", RecipientUserId: "U1", EventType: "chat.message",
			PayloadJson: []byte(`{"message_id":"M1"}`), OrderingKey: "user:U1",
			Mode: deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
		}},
	}
	if err := ValidateNodeBatch(batch); err == nil {
		t.Fatal("expected missing connection validation error")
	}
	batch.Items[0].ConnectionIds = []string{"C1", "C1"}
	if err := ValidateNodeBatch(batch); err == nil {
		t.Fatal("expected duplicate connection validation error")
	}
}

func TestValidateAckRequiresBackpressureRetrySignal(t *testing.T) {
	t.Parallel()

	ack := &deliveryv1.DeliveryAck{
		ContractVersion: ContractVersion, BatchId: "NB1",
		Status:         deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_PARTIAL,
		AcknowledgedAt: timestamppb.New(time.Unix(2, 0).UTC()),
		Results: []*deliveryv1.DeliveryResult{{
			DeliveryId: "D1", Status: deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_BACKPRESSURED,
		}},
	}
	if err := ValidateAck(ack); err == nil {
		t.Fatal("expected missing retry signal validation error")
	}
	ack.Results[0].RetryAfterMs = 25
	ack.Results[0].ErrorCode = deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL
	ack.Pressure = &deliveryv1.QueuePressure{Depth: 10, Capacity: 10, RetryAfterMs: 25}
	if err := ValidateAck(ack); err != nil {
		t.Fatalf("validate backpressure ack: %v", err)
	}
}

func TestValidateNodeObservationSeparatesShadowFromDeliveryAck(t *testing.T) {
	t.Parallel()

	observation := &deliveryv1.NodeDeliveryObservation{
		ContractVersion:     ContractVersion,
		BatchId:             "NB1",
		TargetNodeId:        "gateway-1",
		Status:              deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_OBSERVED,
		ObservedItems:       2,
		ObservedConnections: 3,
		ObservedAt:          timestamppb.New(time.Unix(3, 0).UTC()),
	}
	if err := ValidateNodeObservation(observation); err != nil {
		t.Fatalf("validate observed response: %v", err)
	}

	observation.Status = deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_BACKPRESSURED
	observation.ObservedItems = 0
	observation.ObservedConnections = 0
	observation.ErrorCode = deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL
	observation.Pressure = &deliveryv1.QueuePressure{Depth: 10, Capacity: 10, RetryAfterMs: 25}
	if err := ValidateNodeObservation(observation); err != nil {
		t.Fatalf("validate backpressured observation: %v", err)
	}

	observation.Pressure = nil
	if err := ValidateNodeObservation(observation); err == nil {
		t.Fatal("expected backpressured observation to require queue pressure")
	}

	observation.Status = deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_REJECTED
	observation.ObservedItems = 0
	observation.ObservedConnections = 0
	observation.ErrorCode = deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM
	if err := ValidateNodeObservation(observation); err != nil {
		t.Fatalf("validate rejected observation: %v", err)
	}
}

func TestValidateBatchRejectsUnsafeBoundaries(t *testing.T) {
	t.Parallel()

	valid := validBatch()
	tests := []struct {
		name   string
		mutate func(*deliveryv1.DeliveryEnvelope)
	}{
		{"version", func(batch *deliveryv1.DeliveryEnvelope) { batch.ContractVersion = "v2" }},
		{"missing source", func(batch *deliveryv1.DeliveryEnvelope) { batch.SourceEventId = "" }},
		{"duplicate delivery id", func(batch *deliveryv1.DeliveryEnvelope) { batch.Items = append(batch.Items, batch.Items[0]) }},
		{"invalid payload", func(batch *deliveryv1.DeliveryEnvelope) { batch.Items[0].PayloadJson = []byte("{") }},
		{"missing ordering key", func(batch *deliveryv1.DeliveryEnvelope) { batch.Items[0].OrderingKey = "" }},
		{"unspecified mode", func(batch *deliveryv1.DeliveryEnvelope) {
			batch.Items[0].Mode = deliveryv1.DeliveryMode_DELIVERY_MODE_UNSPECIFIED
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			batch := cloneBatch(valid)
			test.mutate(batch)
			if err := ValidateEnvelope(batch); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

type recordingSender struct {
	users []string
	types []string
	data  []json.RawMessage
}

func (s *recordingSender) SendEventToUserContext(_ context.Context, userID, eventType string, data any) int {
	s.users = append(s.users, userID)
	s.types = append(s.types, eventType)
	raw, _ := data.(json.RawMessage)
	s.data = append(s.data, raw)
	if userID == "offline" {
		return 0
	}
	return 2
}

func TestLegacyDispatcherProducesPerItemAck(t *testing.T) {
	sender := &recordingSender{}
	dispatcher := NewLegacyDispatcher(sender)
	batch := validBatch()
	batch.Items = append(batch.Items, &deliveryv1.DeliveryItem{
		DeliveryId: "D2", RecipientUserId: "offline", EventType: "chat.message",
		PayloadJson: []byte(`{"message_id":"M2"}`), OrderingKey: "user:offline",
		Mode: deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
	})

	ack, err := dispatcher.Deliver(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Status != deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED {
		t.Fatalf("ack status = %s", ack.Status)
	}
	if len(ack.Results) != 2 || ack.Results[0].AcceptedConnections != 2 || ack.Results[1].Status != deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_OFFLINE {
		t.Fatalf("unexpected results: %+v", ack.Results)
	}
	if err := ValidateAck(ack); err != nil {
		t.Fatalf("validate legacy ack: %v", err)
	}
	if string(sender.data[0]) != `{"message_id":"M1"}` {
		t.Fatalf("payload = %s", sender.data[0])
	}
}

func validBatch() *deliveryv1.DeliveryEnvelope {
	return &deliveryv1.DeliveryEnvelope{
		ContractVersion: ContractVersion, BatchId: "B1", SourceEventId: "E1", SourceTopic: "message.direct.created",
		CreatedAt: timestamppb.New(time.Unix(1, 0).UTC()),
		Items: []*deliveryv1.DeliveryItem{{
			DeliveryId: "D1", RecipientUserId: "U1", EventType: "chat.message",
			PayloadJson: []byte(`{"message_id":"M1"}`), OrderingKey: "user:U1",
			Mode: deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
		}},
	}
}

func cloneBatch(batch *deliveryv1.DeliveryEnvelope) *deliveryv1.DeliveryEnvelope {
	raw, _ := json.Marshal(batch)
	var clone deliveryv1.DeliveryEnvelope
	_ = json.Unmarshal(raw, &clone)
	return &clone
}
