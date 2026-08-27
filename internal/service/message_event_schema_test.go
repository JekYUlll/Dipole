package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

type messageEventSchema struct {
	Required   []string                      `json:"required"`
	Properties map[string]json.RawMessage    `json:"properties"`
	Defs       map[string]messageEventSchema `json:"$defs"`
}

func TestMessageEventSchemaMatchesProducerContract(t *testing.T) {
	t.Parallel()

	schema := loadMessageEventSchema(t, "message-event.schema.json")
	eventTypes := schemaEnum(t, schema.Properties["event_type"])
	wantEventTypes := allMessageEventTypes(t)
	if !reflect.DeepEqual(eventTypes, wantEventTypes) {
		t.Fatalf("schema event types = %v, want %v", eventTypes, wantEventTypes)
	}
	if pattern := schemaPattern(t, schema.Properties["version"]); !regexp.MustCompile(pattern).MatchString(platformKafka.DefaultEventVersion) {
		t.Fatalf("schema version pattern %q rejects producer version %q", pattern, platformKafka.DefaultEventVersion)
	}

	message := &model.Message{
		UUID: "M1", ClientMessageID: "C1", ConversationKey: "direct:U1:U2", Seq: 1,
		SenderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		MessageType: 1, Content: "contract", SentAt: time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC),
	}
	outbox, err := buildMessageCreatedOutboxEvent(message, []string{"U1", "U2"}, true)
	if err != nil {
		t.Fatalf("build Message event: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(outbox.Value, &envelope); err != nil {
		t.Fatalf("decode produced envelope: %v", err)
	}
	assertRequiredFields(t, "envelope", schema.Required, envelope)
	payload, ok := envelope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("produced payload has type %T", envelope["payload"])
	}
	assertRequiredFields(t, "payload", schema.Defs["message_payload"].Required, payload)
}

func TestMessageSendRequestedSchemaMatchesProducerContract(t *testing.T) {
	t.Parallel()

	schema := loadMessageEventSchema(t, "message-send-requested.schema.json")
	eventTypes := schemaEnum(t, schema.Properties["event_type"])
	wantEventTypes := []string{"message.direct.send_requested", "message.group.send_requested"}
	if !reflect.DeepEqual(eventTypes, wantEventTypes) {
		t.Fatalf("request schema event types = %v, want %v", eventTypes, wantEventTypes)
	}

	message := &model.Message{
		UUID: "M1", ClientMessageID: "C1", ConversationKey: "direct:U1:U2",
		SenderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		MessageType: 1, Content: "request", SentAt: time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC),
	}
	envelope, err := platformKafka.NewEnvelope("message.direct.send_requested", messageToEventPayload(message, nil, true))
	if err != nil {
		t.Fatalf("build Message request: %v", err)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("encode Message request: %v", err)
	}
	var produced map[string]any
	if err := json.Unmarshal(raw, &produced); err != nil {
		t.Fatalf("decode produced request: %v", err)
	}
	assertRequiredFields(t, "request envelope", schema.Required, produced)
	payload, ok := produced["payload"].(map[string]any)
	if !ok {
		t.Fatalf("produced request payload has type %T", produced["payload"])
	}
	assertRequiredFields(t, "request payload", schema.Defs["message_request_payload"].Required, payload)
}

func loadMessageEventSchema(t *testing.T, name string) messageEventSchema {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate schema test source")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "contracts", "events", "message", "v1", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Message event schema: %v", err)
	}
	var schema messageEventSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode Message event schema: %v", err)
	}
	return schema
}

func schemaEnum(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var property struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("decode schema enum: %v", err)
	}
	sort.Strings(property.Enum)
	return property.Enum
}

func schemaPattern(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var property struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("decode schema pattern: %v", err)
	}
	return property.Pattern
}

func allMessageEventTypes(t *testing.T) []string {
	t.Helper()
	var eventTypes []string
	for _, target := range []int8{model.MessageTargetDirect, model.MessageTargetGroup} {
		for _, mutation := range []MessageMutationType{MessageMutationCreated, MessageMutationEdited, MessageMutationRecalled, MessageMutationDeleted} {
			eventType, err := MessageMutationEventType(target, mutation)
			if err != nil {
				t.Fatalf("build Message event type: %v", err)
			}
			eventTypes = append(eventTypes, eventType)
		}
	}
	sort.Strings(eventTypes)
	return eventTypes
}

func assertRequiredFields(t *testing.T, scope string, required []string, value map[string]any) {
	t.Helper()
	for _, field := range required {
		if _, ok := value[field]; !ok {
			t.Errorf("%s producer omitted schema-required field %q", scope, field)
		}
	}
}
