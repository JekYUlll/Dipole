package storage

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestAgentArtifactBlobStoreIsContentAddressedAndBucketBound(t *testing.T) {
	client := &agentArtifactObjectClientStubV1{objects: map[string][]byte{}}
	store := newAgentArtifactBlobStoreV1(client, "artifacts")
	body := []byte("report")
	artifact, err := application.NewAgentArtifactV1(application.AgentArtifactCreateV1{TenantID: "dipole", TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "report", Version: 1, Title: "Report", MediaType: "text/plain", Content: body})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.PutImmutable(context.Background(), artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256)
	if err != nil || receipt.Bucket != "artifacts" || client.puts != 1 {
		t.Fatalf("put receipt=%+v puts=%d err=%v", receipt, client.puts, err)
	}
	if _, err := store.PutImmutable(context.Background(), artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256); err != nil || client.puts != 1 {
		t.Fatalf("exact replay should reuse object: puts=%d err=%v", client.puts, err)
	}
	client.objects["artifacts\n"+artifact.ObjectKey] = []byte("drift")
	if _, err := store.PutImmutable(context.Background(), artifact.ObjectKey, artifact.MediaType, body, artifact.ContentSHA256); err == nil {
		t.Fatal("expected content-address collision to fail")
	}
	if _, err := store.Open(context.Background(), application.AgentArtifactBlobReceiptV1{Bucket: "other", ObjectKey: artifact.ObjectKey}); err == nil {
		t.Fatal("expected receipt bucket mismatch to fail")
	}
}

func TestNewAgentArtifactBlobStoreRequiresDedicatedConfiguration(t *testing.T) {
	for name, cfg := range map[string]AgentArtifactStorageConfigV1{
		"disabled":        {},
		"missing key":     {Enabled: true, Endpoint: "minio:9000", SecretKey: "secret", Bucket: "dipole-agent-artifacts"},
		"missing secret":  {Enabled: true, Endpoint: "minio:9000", AccessKey: "artifact", Bucket: "dipole-agent-artifacts"},
		"shared identity": {Enabled: true, Endpoint: "minio:9000", AccessKey: "shared", SecretKey: "secret", Bucket: "artifacts", GeneralAccessKey: "shared", GeneralBucket: "files"},
		"shared bucket":   {Enabled: true, Endpoint: "minio:9000", AccessKey: "artifact", SecretKey: "secret", Bucket: "shared", GeneralAccessKey: "files", GeneralBucket: "shared"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAgentArtifactBlobStoreFromConfig(context.Background(), cfg); err == nil {
				t.Fatal("expected isolated Agent Artifact configuration to be rejected")
			}
		})
	}
}

func TestNewAgentArtifactObjectSourceRequiresSeparateReadOnlyIdentity(t *testing.T) {
	for name, cfg := range map[string]AgentArtifactAuditConfigV1{
		"missing": {},
		"shared runtime identity": {
			Endpoint: "minio:9000", AccessKey: "shared", SecretKey: "secret",
			Bucket: "dipole-agent-artifacts", RuntimeAccessKey: "shared",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAgentArtifactObjectSourceV1(context.Background(), cfg); err == nil {
				t.Fatal("expected unsafe Artifact audit identity to be rejected")
			}
		})
	}
}

func TestNewAgentArtifactMaintenanceInspectorRequiresThirdIdentity(t *testing.T) {
	for name, cfg := range map[string]AgentArtifactMaintenanceConfigV1{
		"missing":       {},
		"runtime reuse": {Endpoint: "minio:9000", AccessKey: "runtime", SecretKey: "secret", Bucket: "artifacts", RuntimeAccessKey: "runtime", AuditAccessKey: "audit"},
		"audit reuse":   {Endpoint: "minio:9000", AccessKey: "audit", SecretKey: "secret", Bucket: "artifacts", RuntimeAccessKey: "runtime", AuditAccessKey: "audit"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAgentArtifactMaintenanceInspectorV1(cfg); err == nil {
				t.Fatal("expected shared maintenance inspection identity to fail")
			}
		})
	}
}

type agentArtifactObjectClientStubV1 struct {
	objects map[string][]byte
	puts    int
}

func (s *agentArtifactObjectClientStubV1) Get(_ context.Context, bucket, key string) ([]byte, bool, error) {
	body, ok := s.objects[bucket+"\n"+key]
	return body, ok, nil
}
func (s *agentArtifactObjectClientStubV1) Put(_ context.Context, bucket, key, _ string, body []byte) error {
	s.puts++
	s.objects[bucket+"\n"+key] = append([]byte(nil), body...)
	return nil
}
