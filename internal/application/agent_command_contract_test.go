package application_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentCommandV1HasLanguageNeutralContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "contracts", "agent-commands", "v1", "schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Agent Command v1 contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatalf("decode Agent Command v1 contract: %v", err)
	}
	if schema["$id"] != "https://dipole.local/contracts/agent-commands/v1/schema.json" {
		t.Fatalf("unexpected Agent Command schema ID %q", schema["$id"])
	}
	if schema["x-dipole-version"] != application.AgentCommandVersionV1 {
		t.Fatalf("schema version = %q, want %q", schema["x-dipole-version"], application.AgentCommandVersionV1)
	}
	kinds, ok := schema["x-dipole-command-kinds"].([]any)
	if !ok || len(kinds) != 2 {
		t.Fatalf("Agent Command contract must declare two kinds, got %#v", schema["x-dipole-command-kinds"])
	}
	want := map[string]bool{
		string(application.AgentMessageCommandAssistantReplyV1): true,
		string(application.AgentMessageCommandSystemMessageV1):  true,
	}
	for _, raw := range kinds {
		kind, ok := raw.(string)
		if !ok || !want[kind] {
			t.Fatalf("unexpected Agent Command kind %#v", raw)
		}
		delete(want, kind)
	}
	if len(want) != 0 {
		t.Fatalf("missing Agent Command kinds: %#v", want)
	}

	descriptors, ok := schema["x-dipole-capabilities"].([]any)
	if !ok || len(descriptors) != 2 {
		t.Fatalf("Agent Command contract must publish two capability descriptors, got %#v", schema["x-dipole-capabilities"])
	}
	wantDescriptors := map[string]application.AgentCapabilityDescriptorV1{}
	for _, id := range []string{application.AgentCapabilityAssistantReplySend, application.AgentCapabilitySystemMessageSend} {
		descriptor, ok := application.AgentCapabilityDescriptorByIDV1(id)
		if !ok {
			t.Fatalf("Go registry is missing descriptor %s", id)
		}
		wantDescriptors[id] = descriptor
	}
	for _, raw := range descriptors {
		descriptor, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid Agent Command descriptor %#v", raw)
		}
		id, _ := descriptor["id"].(string)
		want, ok := wantDescriptors[id]
		if !ok {
			t.Fatalf("schema has unknown Agent Command descriptor %q", id)
		}
		if descriptor["risk"] != string(want.Risk) || descriptor["required_permission"] != want.RequiredPermission || descriptor["approval_required"] != want.ApprovalRequired {
			t.Fatalf("schema descriptor drift for %s: schema=%#v Go=%+v", id, descriptor, want)
		}
		delete(wantDescriptors, id)
	}
	if len(wantDescriptors) != 0 {
		t.Fatalf("schema is missing Agent Command descriptors: %#v", wantDescriptors)
	}
}

func TestAgentMessageCommandV1SerializesWithContractFieldNames(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(application.AgentMessageCommandV1{
		CommandID: "C1",
		Kind:      application.AgentMessageCommandAssistantReplyV1,
		Invocation: application.AgentInvocationV1{
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
			Permissions: []string{application.AgentPermissionMessageWrite},
		},
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("marshal Agent Message Command v1: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode Agent Message Command v1: %v", err)
	}
	if envelope["command_id"] != "C1" || envelope["kind"] != string(application.AgentMessageCommandAssistantReplyV1) {
		t.Fatalf("unexpected command field names: %s", payload)
	}
	invocation, ok := envelope["invocation"].(map[string]any)
	if !ok || invocation["tenant_id"] != "dipole" || invocation["principal_uuid"] != "U100" || invocation["agent_uuid"] != "UAI" {
		t.Fatalf("unexpected invocation field names: %s", payload)
	}
}
