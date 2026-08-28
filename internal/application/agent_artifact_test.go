package application

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewAgentArtifactV1IsDeterministicAndCanonicalizesMetadata(t *testing.T) {
	content := []byte("# Project report\n")
	first, err := NewAgentArtifactV1(AgentArtifactCreateV1{
		TaskUUID: "task:1", RunUUID: "run:1", ArtifactType: "project_report", Version: 1,
		Title: "Project report", MediaType: "text/markdown", Content: content,
		Metadata: json.RawMessage(`{"z":1,"a":"source"}`), TenantID: "dipole",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAgentArtifactV1(AgentArtifactCreateV1{
		TaskUUID: "task:1", RunUUID: "run:1", ArtifactType: "project_report", Version: 1,
		Title: "Project report", MediaType: "text/markdown", Content: content,
		Metadata: json.RawMessage(`{"a":"source","z":1}`), TenantID: "dipole",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ArtifactUUID != second.ArtifactUUID || string(first.Metadata) != `{"a":"source","z":1}` {
		t.Fatalf("artifact identity or metadata is not canonical: first=%+v second=%+v", first, second)
	}
	if len(first.ArtifactUUID) != 64 || len(first.ContentSHA256) != 64 || !strings.Contains(first.ObjectKey, first.ContentSHA256) {
		t.Fatalf("unexpected immutable evidence: %+v", first)
	}
}

func TestNewAgentArtifactV1RejectsUnsafeOrUnboundedInput(t *testing.T) {
	base := AgentArtifactCreateV1{TaskUUID: "task:1", RunUUID: "run:1", ArtifactType: "report", Version: 1, Title: "Report", MediaType: "text/plain", Content: []byte("ok"), Metadata: json.RawMessage(`{}`), TenantID: "dipole"}
	tests := []AgentArtifactCreateV1{
		{TaskUUID: base.TaskUUID, RunUUID: base.RunUUID, ArtifactType: "../report", Version: 1, Title: base.Title, MediaType: base.MediaType, Content: base.Content, Metadata: base.Metadata, TenantID: base.TenantID},
		{TaskUUID: base.TaskUUID, RunUUID: base.RunUUID, ArtifactType: base.ArtifactType, Version: 0, Title: base.Title, MediaType: base.MediaType, Content: base.Content, Metadata: base.Metadata, TenantID: base.TenantID},
		{TaskUUID: base.TaskUUID, RunUUID: base.RunUUID, ArtifactType: base.ArtifactType, Version: 1, Title: base.Title, MediaType: base.MediaType, Content: nil, Metadata: base.Metadata, TenantID: base.TenantID},
		{TaskUUID: base.TaskUUID, RunUUID: base.RunUUID, ArtifactType: base.ArtifactType, Version: 1, Title: base.Title, MediaType: base.MediaType, Content: make([]byte, AgentArtifactMaxBodyBytesV1+1), Metadata: base.Metadata, TenantID: base.TenantID},
		{TaskUUID: base.TaskUUID, RunUUID: base.RunUUID, ArtifactType: base.ArtifactType, Version: 1, Title: base.Title, MediaType: base.MediaType, Content: base.Content, Metadata: json.RawMessage(`[]`), TenantID: base.TenantID},
	}
	for _, input := range tests {
		if _, err := NewAgentArtifactV1(input); err == nil {
			t.Fatalf("expected invalid artifact input to fail: %+v", input)
		}
	}
}
