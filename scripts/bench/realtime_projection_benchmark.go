package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	deliveryv1 "github.com/JekYUlll/Dipole/api/gen/go/delivery/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type event struct {
	EventID   string  `json:"event_id"`
	RequestID string  `json:"request_id"`
	TraceID   string  `json:"trace_id"`
	EventType string  `json:"event_type"`
	Version   string  `json:"version"`
	Source    string  `json:"source"`
	Occurred  string  `json:"occurred_at"`
	Payload   payload `json:"payload"`
}

type payload struct {
	MessageID       string `json:"message_id"`
	ConversationKey string `json:"conversation_key"`
	MessageSeq      uint64 `json:"message_seq"`
	SenderUUID      string `json:"sender_uuid"`
	TargetUUID      string `json:"target_uuid"`
	TargetType      int32  `json:"target_type"`
	MessageType     int32  `json:"message_type"`
	Content         string `json:"content"`
	SentAt          string `json:"sent_at"`
}

type report struct {
	SchemaVersion string  `json:"schema_version"`
	Language      string  `json:"language"`
	Iterations    uint64  `json:"iterations"`
	ItemCount     uint64  `json:"item_count"`
	ElapsedNS     int64   `json:"elapsed_ns"`
	OpsPerSecond  float64 `json:"ops_per_second"`
}

const rawEvent = `{"event_id":"E-benchmark","request_id":"R-benchmark","trace_id":"T-benchmark","event_type":"message.direct.created","version":"v1","source":"dipole","occurred_at":"2026-08-29T00:00:00Z","payload":{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M-benchmark","conversation_key":"direct:U1:U2","message_seq":42,"sender_uuid":"U1","target_uuid":"U2","target_type":0,"message_type":0,"content":"benchmark body","sent_at":"2026-08-29T00:00:00Z"}}`

func parseIterations() uint64 {
	if len(os.Args) < 2 {
		return 100000
	}
	parsed, err := strconv.ParseUint(os.Args[1], 10, 64)
	if err != nil || parsed == 0 {
		return 100000
	}
	return parsed
}

func project(raw []byte) (*deliveryv1.DeliveryEnvelope, error) {
	var input event
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	if input.EventType != "message.direct.created" || input.Version != "v1" || input.Source != "dipole" {
		return nil, fmt.Errorf("unsupported benchmark event")
	}
	occurredAt, err := time.Parse(time.RFC3339, input.Occurred)
	if err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, input.Payload.SentAt); err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(map[string]any{
		"message_id":   input.Payload.MessageID,
		"from_uuid":    input.Payload.SenderUUID,
		"target_uuid":  input.Payload.TargetUUID,
		"target_type":  input.Payload.TargetType,
		"message_type": input.Payload.MessageType,
		"content":      input.Payload.Content,
		"sent_at":      input.Payload.SentAt,
		"message_seq":  input.Payload.MessageSeq,
	})
	if err != nil {
		return nil, err
	}
	return &deliveryv1.DeliveryEnvelope{
		ContractVersion: "v1",
		BatchId:         "shadow:" + input.EventID + ":0:0",
		SourceEventId:   input.EventID,
		SourceTopic:     "dipole.message.created",
		RequestId:       input.RequestID,
		TraceId:         input.TraceID,
		CreatedAt:       timestamppb.New(occurredAt),
		Items: []*deliveryv1.DeliveryItem{{
			DeliveryId:      input.EventID + ":" + input.Payload.TargetUUID + ":full",
			RecipientUserId: input.Payload.TargetUUID,
			EventType:       "chat.message",
			PayloadJson:     payloadJSON,
			OrderingKey:     "user:" + input.Payload.TargetUUID,
			Mode:            deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
		}},
	}, nil
}

func main() {
	iterations := parseIterations()
	raw := []byte(rawEvent)
	for i := 0; i < 1000; i++ {
		if _, err := project(raw); err != nil {
			panic(err)
		}
	}
	var itemCount uint64
	started := time.Now()
	for i := uint64(0); i < iterations; i++ {
		output, err := project(raw)
		if err != nil || len(output.GetItems()) != 1 {
			panic(fmt.Sprintf("projection failed: %v", err))
		}
		itemCount += uint64(len(output.GetItems()))
	}
	elapsed := time.Since(started)
	result := report{
		SchemaVersion: "dipole.realtime.projection-benchmark.v1",
		Language:      "go",
		Iterations:    iterations,
		ItemCount:     itemCount,
		ElapsedNS:     elapsed.Nanoseconds(),
		OpsPerSecond:  float64(iterations) / elapsed.Seconds(),
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
