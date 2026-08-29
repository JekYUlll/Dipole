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

	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	corecontact "github.com/JekYUlll/Dipole/internal/services/core/domain/contact"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

func TestDomainEventSchemasMatchProducerContracts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 27, 2, 3, 4, 0, time.UTC)
	tests := []struct {
		name       string
		schemaFile string
		eventType  string
		eventTypes []string
		definition string
		payload    any
	}{
		{
			name: "group", schemaFile: "group-event.schema.json", eventType: "group.created",
			eventTypes: []string{"group.created", "group.dismissed", "group.members.added", "group.members.removed", "group.updated"},
			definition: "group_payload",
			payload: GroupEventPayload{
				GroupUUID: "G1", Name: "Group", OperatorUUID: "U1",
				MemberUUIDs: []string{"U1"}, RecipientUUIDs: []string{"U1"}, OccurredAt: now,
			},
		},
		{
			name: "conversation read", schemaFile: "conversation-direct-read.schema.json", eventType: "conversation.direct.read",
			eventTypes: []string{"conversation.direct.read"}, definition: "read_payload",
			payload: coreconversation.ConversationReadReceipt{
				ReaderUUID: "U1", TargetUUID: "U2", ConversationKey: "direct:U1:U2",
				LastReadMessageUUID: "M1", LastReadSeq: 3, ReadAt: now,
			},
		},
		{
			name: "contact deleted", schemaFile: "contact-friend-deleted.schema.json", eventType: "contact.friend.deleted",
			eventTypes: []string{"contact.friend.deleted"}, definition: "contact_payload",
			payload: corecontact.ContactFriendDeletedPayload{UserUUID: "U1", FriendUUID: "U2", OccurredAt: now},
		},
		{
			name: "session logout", schemaFile: "session-force-logout.schema.json", eventType: "session.force_logout",
			eventTypes: []string{"session.force_logout"}, definition: "session_payload",
			payload: coresession.SessionKickEventPayload{UserUUID: "U1", All: true, Reason: "forced_logout_all", OccurredAt: now},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := loadDomainEventSchema(t, test.schemaFile)
			if got := schemaEventTypes(t, schema.Properties["event_type"]); !reflect.DeepEqual(got, test.eventTypes) {
				t.Fatalf("schema event types = %v, want %v", got, test.eventTypes)
			}
			if pattern := schemaPattern(t, schema.Properties["version"]); !regexp.MustCompile(pattern).MatchString(platformKafka.DefaultEventVersion) {
				t.Fatalf("schema version pattern %q rejects %q", pattern, platformKafka.DefaultEventVersion)
			}

			produced := produceDomainEnvelope(t, test.eventType, test.payload)
			assertRequiredFields(t, "envelope", schema.Required, produced)
			payload, ok := produced["payload"].(map[string]any)
			if !ok {
				t.Fatalf("produced payload has type %T", produced["payload"])
			}
			assertRequiredFields(t, "payload", schema.Defs[test.definition].Required, payload)
		})
	}
}

func loadDomainEventSchema(t *testing.T, name string) messageEventSchema {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate domain schema test source")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "contracts", "events", "domain", "v1", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read domain event schema: %v", err)
	}
	var schema messageEventSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode domain event schema: %v", err)
	}
	return schema
}

func schemaEventTypes(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var property struct {
		Const string   `json:"const"`
		Enum  []string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("decode schema event type: %v", err)
	}
	if property.Const != "" {
		property.Enum = append(property.Enum, property.Const)
	}
	sort.Strings(property.Enum)
	return property.Enum
}

func produceDomainEnvelope(t *testing.T, eventType string, payload any) map[string]any {
	t.Helper()
	envelope, err := platformKafka.NewEnvelope(eventType, payload)
	if err != nil {
		t.Fatalf("build domain event: %v", err)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("encode domain event: %v", err)
	}
	var produced map[string]any
	if err := json.Unmarshal(raw, &produced); err != nil {
		t.Fatalf("decode produced domain event: %v", err)
	}
	return produced
}
