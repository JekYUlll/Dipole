package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/JekYUlll/Dipole/internal/service"
)

type event struct {
	EventID   string                       `json:"event_id"`
	EventType string                       `json:"event_type"`
	Payload   service.MessageEventPayload `json:"payload"`
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

func main() {
	iterations := uint64(100000)
	if len(os.Args) > 1 {
		if parsed, err := strconv.ParseUint(os.Args[1], 10, 64); err == nil && parsed > 0 {
			iterations = parsed
		}
	}
	var input event
	if err := json.Unmarshal([]byte(rawEvent), &input); err != nil {
		panic(err)
	}
	for i := 0; i < 1000; i++ {
		var decoded event
		if err := json.Unmarshal([]byte(rawEvent), &decoded); err != nil {
			panic(err)
		}
		if _, _, err := service.MessageSyncProjection(decoded.EventID, decoded.EventType, decoded.Payload); err != nil {
			panic(err)
		}
	}
	var itemCount uint64
	started := time.Now()
	for i := uint64(0); i < iterations; i++ {
		var decoded event
		if err := json.Unmarshal([]byte(rawEvent), &decoded); err != nil {
			panic(err)
		}
		projection, fanout, err := service.MessageSyncProjection(decoded.EventID, decoded.EventType, decoded.Payload)
		if err != nil || !fanout || projection == nil {
			panic(fmt.Sprintf("projection failed: %v", err))
		}
		itemCount++
	}
	elapsed := time.Since(started)
	result := report{
		SchemaVersion: "dipole.realtime.projection-benchmark.v1",
		Language: "go",
		Iterations: iterations,
		ItemCount: itemCount,
		ElapsedNS: elapsed.Nanoseconds(),
		OpsPerSecond: float64(iterations) / elapsed.Seconds(),
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
