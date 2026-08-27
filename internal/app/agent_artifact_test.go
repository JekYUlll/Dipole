package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestPersistentAgentArtifactCreateConvergesAndRejectsVersionDrift(t *testing.T) {
	store := newAgentArtifactStoreStubV1()
	blobs := &agentArtifactBlobStoreStubV1{bucket: "dipole-agent-artifacts", bodies: map[string][]byte{}}
	service, _ := NewPersistentAgentArtifactServiceV1(agentArtifactPolicyStubV1{
		task: &application.AgentTaskV1{TaskUUID: "TASK-1", TenantID: "dipole", PrincipalUUID: "U1"},
		run:  &application.AgentRunV1{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning},
	}, store, blobs)
	input := application.AgentArtifactCreateV1{TenantID: "dipole", TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "project_report", Version: 1, Title: "Report", MediaType: "text/markdown", Content: []byte("report"), Metadata: json.RawMessage(`{"source":"G1"}`)}
	first, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(context.Background(), input)
	if err != nil || second.ArtifactUUID != first.ArtifactUUID || blobs.puts != 1 {
		t.Fatalf("exact replay did not converge: second=%+v puts=%d err=%v", second, blobs.puts, err)
	}
	input.Content = []byte("drift")
	if _, err := service.Create(context.Background(), input); !errors.Is(err, application.ErrAgentArtifactConflict) {
		t.Fatalf("expected immutable version conflict, got %v", err)
	}
	input.Content = []byte("report")
	input.TenantID = "other"
	if _, err := service.Create(context.Background(), input); !errors.Is(err, application.ErrAgentArtifactDenied) {
		t.Fatalf("expected exact replay with tenant drift to fail, got %v", err)
	}
	input.TenantID = "dipole"
	blobs.bodies[first.ObjectKey] = []byte("tampered")
	if _, err := service.Create(context.Background(), input); !errors.Is(err, application.ErrAgentArtifactConflict) {
		t.Fatalf("expected exact replay to verify the stored object, got %v", err)
	}
}

func TestPersistentAgentArtifactCreateRequiresActiveBoundShadowRun(t *testing.T) {
	for _, run := range []*application.AgentRunV1{
		{RunUUID: "RUN-1", TaskUUID: "TASK-2", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning},
		{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "other", Mode: "shadow", Status: application.AgentRunStatusRunning},
		{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning},
		{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusCompleted},
	} {
		service, _ := NewPersistentAgentArtifactServiceV1(agentArtifactPolicyStubV1{
			task: &application.AgentTaskV1{TaskUUID: "TASK-1", TenantID: "dipole", PrincipalUUID: "U1"}, run: run,
		}, newAgentArtifactStoreStubV1(), &agentArtifactBlobStoreStubV1{bucket: "bucket", bodies: map[string][]byte{}})
		_, err := service.Create(context.Background(), artifactCreateInputV1())
		if !errors.Is(err, application.ErrAgentArtifactDenied) {
			t.Fatalf("expected denied binding for %+v, got %v", run, err)
		}
	}
}

func TestPersistentAgentArtifactRetrievalEnforcesPrincipalAndContentEvidence(t *testing.T) {
	store := newAgentArtifactStoreStubV1()
	blobs := &agentArtifactBlobStoreStubV1{bucket: "bucket", bodies: map[string][]byte{}}
	service, _ := NewPersistentAgentArtifactServiceV1(agentArtifactPolicyStubV1{
		task: &application.AgentTaskV1{TaskUUID: "TASK-1", TenantID: "dipole", PrincipalUUID: "U1"},
		run:  &application.AgentRunV1{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning},
	}, store, blobs)
	artifact, err := service.Create(context.Background(), artifactCreateInputV1())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.GetForPrincipal(context.Background(), "U2", artifact.ArtifactUUID); !errors.Is(err, application.ErrAgentArtifactDenied) {
		t.Fatalf("expected cross-principal retrieval denial, got %v", err)
	}
	_, body, err := service.GetForPrincipal(context.Background(), "U1", artifact.ArtifactUUID)
	if err != nil || string(body) != "report" {
		t.Fatalf("authorized retrieval failed: body=%q err=%v", body, err)
	}
	blobs.bodies[artifact.ObjectKey] = []byte("tampered")
	if _, _, err := service.GetForPrincipal(context.Background(), "U1", artifact.ArtifactUUID); !errors.Is(err, application.ErrAgentArtifactConflict) {
		t.Fatalf("expected content evidence conflict, got %v", err)
	}
}

func artifactCreateInputV1() application.AgentArtifactCreateV1 {
	return application.AgentArtifactCreateV1{TenantID: "dipole", TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "project_report", Version: 1, Title: "Report", MediaType: "text/markdown", Content: []byte("report"), Metadata: json.RawMessage(`{}`)}
}

type agentArtifactPolicyStubV1 struct {
	task *application.AgentTaskV1
	run  *application.AgentRunV1
}

func (s agentArtifactPolicyStubV1) GetTask(context.Context, string) (*application.AgentTaskV1, error) {
	return s.task, nil
}
func (s agentArtifactPolicyStubV1) GetRun(context.Context, string) (*application.AgentRunV1, error) {
	return s.run, nil
}

type agentArtifactStoreStubV1 struct {
	byID      map[string]*application.AgentArtifactV1
	byVersion map[string]*application.AgentArtifactV1
}

func newAgentArtifactStoreStubV1() *agentArtifactStoreStubV1 {
	return &agentArtifactStoreStubV1{byID: map[string]*application.AgentArtifactV1{}, byVersion: map[string]*application.AgentArtifactV1{}}
}
func artifactVersionKeyV1(a application.AgentArtifactV1) string {
	return a.TaskUUID + "\n" + a.ArtifactType + "\n" + strconv.FormatUint(uint64(a.Version), 10)
}
func (s *agentArtifactStoreStubV1) CreateAgentArtifact(_ context.Context, a application.AgentArtifactV1) (bool, error) {
	if _, ok := s.byID[a.ArtifactUUID]; ok {
		return false, nil
	}
	copy := a
	s.byID[a.ArtifactUUID] = &copy
	s.byVersion[artifactVersionKeyV1(a)] = &copy
	return true, nil
}
func (s *agentArtifactStoreStubV1) GetAgentArtifact(_ context.Context, id string) (*application.AgentArtifactV1, error) {
	return s.byID[id], nil
}
func (s *agentArtifactStoreStubV1) GetAgentArtifactByTaskTypeVersion(_ context.Context, task, kind string, version uint32) (*application.AgentArtifactV1, error) {
	return s.byVersion[artifactVersionKeyV1(application.AgentArtifactV1{TaskUUID: task, ArtifactType: kind, Version: version})], nil
}

type agentArtifactBlobStoreStubV1 struct {
	bucket string
	bodies map[string][]byte
	puts   int
}

func (s *agentArtifactBlobStoreStubV1) PutImmutable(_ context.Context, key, _ string, body []byte, _ string) (application.AgentArtifactBlobReceiptV1, error) {
	s.puts++
	s.bodies[key] = bytes.Clone(body)
	return application.AgentArtifactBlobReceiptV1{Bucket: s.bucket, ObjectKey: key}, nil
}
func (s *agentArtifactBlobStoreStubV1) Open(_ context.Context, receipt application.AgentArtifactBlobReceiptV1) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.bodies[receipt.ObjectKey])), nil
}
