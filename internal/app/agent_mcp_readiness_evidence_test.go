package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestPersistentAgentMCPReadinessEvidencePublisherV1PublishesAndReplays(t *testing.T) {
	store := &agentMCPReadinessEvidenceStoreStubV1{created: true}
	publisher, err := NewPersistentAgentMCPReadinessEvidencePublisherV1(store)
	if err != nil {
		t.Fatal(err)
	}
	request := readinessEvidencePublishRequest()
	record, created, err := publisher.PublishAgentMCPReadinessEvidence(context.Background(), "OPERATOR", request)
	if err != nil || !created || record == nil || store.appended == nil || record.EvidenceUUID != store.appended.EvidenceUUID {
		t.Fatalf("publish: record=%+v created=%t err=%v", record, created, err)
	}
	store.created = false
	replayed, created, err := publisher.PublishAgentMCPReadinessEvidence(context.Background(), "OPERATOR", request)
	if err != nil || created || replayed == nil || replayed.EvidenceUUID != record.EvidenceUUID {
		t.Fatalf("replay: record=%+v created=%t err=%v", replayed, created, err)
	}
}

func TestPersistentAgentMCPReadinessEvidencePublisherV1FailsBeforeOrDuringStorage(t *testing.T) {
	store := &agentMCPReadinessEvidenceStoreStubV1{created: true}
	publisher, _ := NewPersistentAgentMCPReadinessEvidencePublisherV1(store)
	invalid := readinessEvidencePublishRequest()
	invalid.Evidence.ToolCount = 0
	if _, _, err := publisher.PublishAgentMCPReadinessEvidence(context.Background(), "OPERATOR", invalid); !errors.Is(err, application.ErrAgentMCPReadinessEvidenceInvalid) || store.appended != nil {
		t.Fatalf("invalid publish: appended=%+v err=%v", store.appended, err)
	}
	want := errors.New("write failed")
	store.err = want
	if _, _, err := publisher.PublishAgentMCPReadinessEvidence(context.Background(), "OPERATOR", readinessEvidencePublishRequest()); !errors.Is(err, want) {
		t.Fatalf("storage error=%v", err)
	}
}

type agentMCPReadinessEvidenceStoreStubV1 struct {
	appended *application.AgentMCPReadinessEvidenceRecordV1
	created  bool
	err      error
}

func (store *agentMCPReadinessEvidenceStoreStubV1) AppendAgentMCPReadinessEvidence(_ context.Context, record application.AgentMCPReadinessEvidenceRecordV1) (bool, error) {
	store.appended = &record
	if store.err != nil {
		return false, store.err
	}
	return store.created, nil
}

func (*agentMCPReadinessEvidenceStoreStubV1) GetAgentMCPReadinessEvidence(context.Context, string, string) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	return nil, nil
}

func (*agentMCPReadinessEvidenceStoreStubV1) GetFreshAgentMCPReadinessEvidence(context.Context, application.AgentMCPReadinessEvidenceLookupV1) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	return nil, nil
}

func readinessEvidencePublishRequest() application.AgentMCPReadinessEvidenceRequestV1 {
	startedAt := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	profileBinding := strings.Repeat("b", 64)
	return application.AgentMCPReadinessEvidenceRequestV1{
		TenantID: "dipole", ProfileBindingSHA256: profileBinding, RequestID: "REQ-1", TraceID: "TRACE-1",
		ExpiresAt: startedAt.Add(30 * time.Minute),
		Evidence: application.AgentMCPReadinessEvidenceV1{
			SchemaVersion: application.AgentMCPReadinessEvidenceSchemaVersionV2,
			BindingSHA256: strings.Repeat("a", 64), ProfileBindingSHA256: profileBinding,
			StartedAt: startedAt, PreflightCheckedAt: startedAt.Add(time.Second),
			ConnectivityCheckedAt: startedAt.Add(2 * time.Second), CompletedAt: startedAt.Add(3 * time.Second),
			ProfileCount: 1, CredentialCount: 1, CABundleCount: 1, ToolCount: 1,
		},
	}
}
