package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	AgentMCPReadinessEvidenceSchemaVersionV2  = "dipole.agent.external-mcp-readiness-evidence.v2"
	AgentMCPReadinessEvidenceRecordSchemaV1   = "dipole.agent.external-mcp-readiness-evidence-record.v1"
	AgentMCPReadinessEvidenceStatusRecordedV1 = "recorded"
)

var (
	ErrAgentMCPReadinessEvidenceInvalid  = errors.New("agent MCP readiness evidence is invalid")
	ErrAgentMCPReadinessEvidenceConflict = errors.New("agent MCP readiness evidence conflicts with immutable history")
)

type AgentMCPReadinessEvidenceV1 struct {
	SchemaVersion         string    `json:"schemaVersion"`
	BindingSHA256         string    `json:"bindingSha256"`
	ProfileBindingSHA256  string    `json:"profileBindingSha256"`
	StartedAt             time.Time `json:"startedAt"`
	CompletedAt           time.Time `json:"completedAt"`
	PreflightCheckedAt    time.Time `json:"preflightCheckedAt"`
	ConnectivityCheckedAt time.Time `json:"connectivityCheckedAt"`
	ProfileCount          uint32    `json:"profileCount"`
	CredentialCount       uint32    `json:"credentialCount"`
	CABundleCount         uint32    `json:"caBundleCount"`
	ToolCount             uint32    `json:"toolCount"`
}

type AgentMCPReadinessEvidenceRequestV1 struct {
	TenantID, ProfileBindingSHA256 string
	RequestID, TraceID             string
	ExpiresAt                      time.Time
	Evidence                       AgentMCPReadinessEvidenceV1
}

type AgentMCPReadinessEvidenceRecordV1 struct {
	EvidenceUUID, SchemaVersion              string
	TenantID, ProfileBindingSHA256           string
	RuntimeBindingSHA256, ContentSHA256      string
	ContentJSON                              json.RawMessage
	OperatorUUID, RequestID, TraceID, Status string
	CollectedAt, ExpiresAt, CreatedAt        time.Time
}

type AgentMCPReadinessEvidenceLookupV1 struct {
	TenantID, ProfileBindingSHA256, RuntimeBindingSHA256 string
	At                                                   time.Time
}

type AgentMCPReadinessEvidenceStoreV1 interface {
	AppendAgentMCPReadinessEvidence(context.Context, AgentMCPReadinessEvidenceRecordV1) (bool, error)
	GetAgentMCPReadinessEvidence(context.Context, string, string) (*AgentMCPReadinessEvidenceRecordV1, error)
	GetFreshAgentMCPReadinessEvidence(context.Context, AgentMCPReadinessEvidenceLookupV1) (*AgentMCPReadinessEvidenceRecordV1, error)
}

type AgentMCPReadinessEvidencePublisherV1 interface {
	PublishAgentMCPReadinessEvidence(context.Context, string, AgentMCPReadinessEvidenceRequestV1) (*AgentMCPReadinessEvidenceRecordV1, bool, error)
}

func (record AgentMCPReadinessEvidenceRecordV1) Validate() error {
	evidence, err := ParseAgentMCPReadinessEvidenceV1(record.ContentJSON)
	if err != nil {
		return err
	}
	rebuilt, err := NewAgentMCPReadinessEvidenceRecordV1(record.OperatorUUID, AgentMCPReadinessEvidenceRequestV1{
		TenantID: record.TenantID, ProfileBindingSHA256: record.ProfileBindingSHA256,
		RequestID: record.RequestID, TraceID: record.TraceID, ExpiresAt: record.ExpiresAt, Evidence: evidence,
	})
	if err != nil {
		return err
	}
	if record.EvidenceUUID != rebuilt.EvidenceUUID || record.SchemaVersion != rebuilt.SchemaVersion ||
		record.RuntimeBindingSHA256 != rebuilt.RuntimeBindingSHA256 || record.ContentSHA256 != rebuilt.ContentSHA256 ||
		record.Status != rebuilt.Status || !bytes.Equal(record.ContentJSON, rebuilt.ContentJSON) ||
		!record.CollectedAt.UTC().Truncate(time.Millisecond).Equal(rebuilt.CollectedAt) {
		return ErrAgentMCPReadinessEvidenceInvalid
	}
	return nil
}

func NewAgentMCPReadinessEvidenceRecordV1(operatorUUID string, request AgentMCPReadinessEvidenceRequestV1) (AgentMCPReadinessEvidenceRecordV1, error) {
	operatorUUID = strings.TrimSpace(operatorUUID)
	if !validReadinessIdentifierV1(operatorUUID, 64) || !validReadinessIdentifierV1(request.TenantID, 64) ||
		!validSHA256V1(request.ProfileBindingSHA256) || request.ProfileBindingSHA256 != request.Evidence.ProfileBindingSHA256 ||
		!validOptionalReadinessIdentifierV1(request.RequestID, 128) || !validOptionalReadinessIdentifierV1(request.TraceID, 128) {
		return AgentMCPReadinessEvidenceRecordV1{}, ErrAgentMCPReadinessEvidenceInvalid
	}
	evidence, content, err := canonicalAgentMCPReadinessEvidenceV1(request.Evidence)
	if err != nil {
		return AgentMCPReadinessEvidenceRecordV1{}, err
	}
	expiresAt := request.ExpiresAt.UTC().Truncate(time.Millisecond)
	if !expiresAt.After(evidence.CompletedAt) || expiresAt.Sub(evidence.CompletedAt) > time.Hour {
		return AgentMCPReadinessEvidenceRecordV1{}, ErrAgentMCPReadinessEvidenceInvalid
	}
	contentDigest := sha256.Sum256(content)
	contentSHA256 := hex.EncodeToString(contentDigest[:])
	identity := strings.Join([]string{
		AgentMCPReadinessEvidenceRecordSchemaV1, request.TenantID, request.ProfileBindingSHA256,
		evidence.BindingSHA256, contentSHA256, operatorUUID, request.RequestID, request.TraceID,
		isoMillisecondsV1(expiresAt),
	}, "\n")
	evidenceDigest := sha256.Sum256([]byte(identity))
	return AgentMCPReadinessEvidenceRecordV1{
		EvidenceUUID: hex.EncodeToString(evidenceDigest[:]), SchemaVersion: AgentMCPReadinessEvidenceRecordSchemaV1,
		TenantID: request.TenantID, ProfileBindingSHA256: request.ProfileBindingSHA256,
		RuntimeBindingSHA256: evidence.BindingSHA256, ContentJSON: content, ContentSHA256: contentSHA256,
		OperatorUUID: operatorUUID, RequestID: request.RequestID, TraceID: request.TraceID,
		Status: AgentMCPReadinessEvidenceStatusRecordedV1, CollectedAt: evidence.CompletedAt, ExpiresAt: expiresAt,
	}, nil
}

func ParseAgentMCPReadinessEvidenceV1(payload []byte) (AgentMCPReadinessEvidenceV1, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var evidence AgentMCPReadinessEvidenceV1
	if err := decoder.Decode(&evidence); err != nil {
		return AgentMCPReadinessEvidenceV1{}, fmt.Errorf("%w: decode evidence", ErrAgentMCPReadinessEvidenceInvalid)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return AgentMCPReadinessEvidenceV1{}, fmt.Errorf("%w: trailing evidence content", ErrAgentMCPReadinessEvidenceInvalid)
	}
	validated, _, err := canonicalAgentMCPReadinessEvidenceV1(evidence)
	return validated, err
}

func canonicalAgentMCPReadinessEvidenceV1(evidence AgentMCPReadinessEvidenceV1) (AgentMCPReadinessEvidenceV1, json.RawMessage, error) {
	evidence.StartedAt = evidence.StartedAt.UTC().Truncate(time.Millisecond)
	evidence.CompletedAt = evidence.CompletedAt.UTC().Truncate(time.Millisecond)
	evidence.PreflightCheckedAt = evidence.PreflightCheckedAt.UTC().Truncate(time.Millisecond)
	evidence.ConnectivityCheckedAt = evidence.ConnectivityCheckedAt.UTC().Truncate(time.Millisecond)
	if evidence.SchemaVersion != AgentMCPReadinessEvidenceSchemaVersionV2 ||
		!validSHA256V1(evidence.BindingSHA256) || !validSHA256V1(evidence.ProfileBindingSHA256) ||
		evidence.StartedAt.IsZero() || evidence.CompletedAt.Before(evidence.StartedAt) || evidence.CompletedAt.Sub(evidence.StartedAt) > 10*time.Minute ||
		evidence.PreflightCheckedAt.Before(evidence.StartedAt) || evidence.PreflightCheckedAt.After(evidence.CompletedAt) ||
		evidence.ConnectivityCheckedAt.Before(evidence.PreflightCheckedAt) || evidence.ConnectivityCheckedAt.After(evidence.CompletedAt) ||
		evidence.ProfileCount < 1 || evidence.ProfileCount > 64 || evidence.CredentialCount < 1 || evidence.CredentialCount > 64 ||
		evidence.CredentialCount > evidence.ProfileCount || evidence.CABundleCount < 1 || evidence.CABundleCount > 64 ||
		evidence.CABundleCount > evidence.ProfileCount || evidence.ToolCount < 1 || evidence.ToolCount > 256 {
		return AgentMCPReadinessEvidenceV1{}, nil, ErrAgentMCPReadinessEvidenceInvalid
	}
	canonical := struct {
		SchemaVersion         string `json:"schemaVersion"`
		BindingSHA256         string `json:"bindingSha256"`
		ProfileBindingSHA256  string `json:"profileBindingSha256"`
		StartedAt             string `json:"startedAt"`
		CompletedAt           string `json:"completedAt"`
		PreflightCheckedAt    string `json:"preflightCheckedAt"`
		ConnectivityCheckedAt string `json:"connectivityCheckedAt"`
		ProfileCount          uint32 `json:"profileCount"`
		CredentialCount       uint32 `json:"credentialCount"`
		CABundleCount         uint32 `json:"caBundleCount"`
		ToolCount             uint32 `json:"toolCount"`
	}{
		evidence.SchemaVersion, evidence.BindingSHA256, evidence.ProfileBindingSHA256,
		isoMillisecondsV1(evidence.StartedAt), isoMillisecondsV1(evidence.CompletedAt),
		isoMillisecondsV1(evidence.PreflightCheckedAt), isoMillisecondsV1(evidence.ConnectivityCheckedAt),
		evidence.ProfileCount, evidence.CredentialCount, evidence.CABundleCount, evidence.ToolCount,
	}
	content, err := json.Marshal(canonical)
	if err != nil {
		return AgentMCPReadinessEvidenceV1{}, nil, fmt.Errorf("marshal Agent MCP readiness evidence: %w", err)
	}
	return evidence, content, nil
}

func validReadinessIdentifierV1(value string, limit int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= limit
}

func validOptionalReadinessIdentifierV1(value string, limit int) bool {
	return value == "" || validReadinessIdentifierV1(value, limit)
}
