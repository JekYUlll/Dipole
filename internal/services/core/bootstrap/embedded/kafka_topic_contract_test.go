package embedded

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKafkaManagedTopicsHaveVersionedContracts(t *testing.T) {
	t.Parallel()

	contracts := []string{
		"contracts/events/message/v1/message-event.schema.json",
		"contracts/events/message/v1/message-send-requested.schema.json",
		"contracts/events/domain/v1/group-event.schema.json",
		"contracts/events/domain/v1/conversation-direct-read.schema.json",
		"contracts/events/domain/v1/contact-friend-deleted.schema.json",
		"contracts/events/domain/v1/session-force-logout.schema.json",
		"contracts/events/agent/v1/agent-task-waiting.schema.json",
	}
	covered := make(map[string]string)
	for _, contract := range contracts {
		for _, eventType := range contractEventTypes(t, contract) {
			if previous := covered[eventType]; previous != "" {
				t.Fatalf("event type %s is declared by both %s and %s", eventType, previous, contract)
			}
			covered[eventType] = contract
		}
	}
	for _, topic := range ManagedKafkaTopics() {
		if covered[topic] == "" {
			t.Errorf("Kafka managed topic %s has no versioned contract", topic)
		}
	}
}

func contractEventTypes(t *testing.T, relativePath string) []string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate Kafka contract test source")
	}
	root := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatalf("read event contract %s: %v", relativePath, err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode event contract %s: %v", relativePath, err)
	}
	var eventType struct {
		Const string   `json:"const"`
		Enum  []string `json:"enum"`
	}
	if err := json.Unmarshal(schema.Properties["event_type"], &eventType); err != nil {
		t.Fatalf("decode event types from %s: %v", relativePath, err)
	}
	if eventType.Const != "" {
		eventType.Enum = append(eventType.Enum, eventType.Const)
	}
	if len(eventType.Enum) == 0 {
		t.Fatalf("event contract %s declares no event types", relativePath)
	}
	return eventType.Enum
}
