package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	AgentArtifactSchemaVersionV1 = "dipole.agent.artifact.v1"
	AgentArtifactMaxBodyBytesV1  = 1 << 20
	AgentArtifactMaxMetadataV1   = 16 << 10
)

var (
	ErrAgentArtifactDenied   = errors.New("agent artifact access denied")
	ErrAgentArtifactConflict = errors.New("agent artifact conflict")
	ErrAgentArtifactInvalid  = errors.New("agent artifact is invalid")
	agentArtifactTypeV1      = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
)

type AgentArtifactV1 struct {
	ArtifactUUID  string
	SchemaVersion string
	TaskUUID      string
	RunUUID       string
	ArtifactType  string
	Version       uint32
	Title         string
	MediaType     string
	ObjectBucket  string
	ObjectKey     string
	ContentSHA256 string
	SizeBytes     uint64
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type AgentArtifactCreateV1 struct {
	TenantID     string
	TaskUUID     string
	RunUUID      string
	ArtifactType string
	Version      uint32
	Title        string
	MediaType    string
	Content      []byte
	Metadata     json.RawMessage
}

type AgentArtifactBlobReceiptV1 struct {
	Bucket    string
	ObjectKey string
}

type AgentArtifactStoreV1 interface {
	CreateAgentArtifact(context.Context, AgentArtifactV1) (bool, error)
	GetAgentArtifact(context.Context, string) (*AgentArtifactV1, error)
	GetAgentArtifactByTaskTypeVersion(context.Context, string, string, uint32) (*AgentArtifactV1, error)
}

type AgentArtifactBlobStoreV1 interface {
	PutImmutable(context.Context, string, string, []byte, string) (AgentArtifactBlobReceiptV1, error)
	Open(context.Context, AgentArtifactBlobReceiptV1) (io.ReadCloser, error)
}

type AgentArtifactServiceV1 interface {
	Create(context.Context, AgentArtifactCreateV1) (*AgentArtifactV1, error)
	GetForPrincipal(context.Context, string, string) (*AgentArtifactV1, []byte, error)
}

func NewAgentArtifactV1(input AgentArtifactCreateV1) (*AgentArtifactV1, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	taskUUID := strings.TrimSpace(input.TaskUUID)
	runUUID := strings.TrimSpace(input.RunUUID)
	artifactType := strings.TrimSpace(input.ArtifactType)
	title := strings.TrimSpace(input.Title)
	mediaType := strings.TrimSpace(input.MediaType)
	if tenantID == "" || len(tenantID) > 64 || taskUUID == "" || len(taskUUID) > 64 || runUUID == "" || len(runUUID) > 64 {
		return nil, fmt.Errorf("%w: tenant, Task, and Run identity are required", ErrAgentArtifactInvalid)
	}
	if !agentArtifactTypeV1.MatchString(artifactType) || input.Version == 0 {
		return nil, fmt.Errorf("%w: type and positive version are required", ErrAgentArtifactInvalid)
	}
	if title == "" || len([]rune(title)) > 200 || mediaType == "" || len(mediaType) > 128 || strings.ContainsAny(mediaType, "\r\n") {
		return nil, fmt.Errorf("%w: title or media type is invalid", ErrAgentArtifactInvalid)
	}
	if len(input.Content) == 0 || len(input.Content) > AgentArtifactMaxBodyBytesV1 {
		return nil, fmt.Errorf("%w: body must contain 1..%d bytes", ErrAgentArtifactInvalid, AgentArtifactMaxBodyBytesV1)
	}
	metadata, err := canonicalArtifactMetadataV1(input.Metadata)
	if err != nil {
		return nil, err
	}
	contentDigest := sha256.Sum256(input.Content)
	contentSHA256 := hex.EncodeToString(contentDigest[:])
	identity := strings.Join([]string{
		AgentArtifactSchemaVersionV1, taskUUID, runUUID, artifactType,
		fmt.Sprintf("%d", input.Version), contentSHA256,
	}, "\n")
	artifactDigest := sha256.Sum256([]byte(identity))
	artifactUUID := hex.EncodeToString(artifactDigest[:])
	objectKey := fmt.Sprintf("agent-artifacts/v1/%s/%s/%s/%s", safeArtifactPathV1(tenantID), safeArtifactPathV1(taskUUID), artifactUUID, contentSHA256)
	return &AgentArtifactV1{
		ArtifactUUID: artifactUUID, SchemaVersion: AgentArtifactSchemaVersionV1,
		TaskUUID: taskUUID, RunUUID: runUUID, ArtifactType: artifactType, Version: input.Version,
		Title: title, MediaType: mediaType, ObjectKey: objectKey, ContentSHA256: contentSHA256,
		SizeBytes: uint64(len(input.Content)), Metadata: metadata,
	}, nil
}

func (a AgentArtifactV1) Validate() error {
	if a.SchemaVersion != AgentArtifactSchemaVersionV1 || len(a.ArtifactUUID) != 64 || len(a.ContentSHA256) != 64 {
		return fmt.Errorf("%w: schema or hash evidence is invalid", ErrAgentArtifactInvalid)
	}
	if _, err := hex.DecodeString(a.ArtifactUUID); err != nil {
		return fmt.Errorf("%w: Artifact ID is invalid", ErrAgentArtifactInvalid)
	}
	if _, err := hex.DecodeString(a.ContentSHA256); err != nil {
		return fmt.Errorf("%w: content hash is invalid", ErrAgentArtifactInvalid)
	}
	if strings.TrimSpace(a.TaskUUID) == "" || strings.TrimSpace(a.RunUUID) == "" || !agentArtifactTypeV1.MatchString(a.ArtifactType) || a.Version == 0 {
		return fmt.Errorf("%w: Artifact binding is invalid", ErrAgentArtifactInvalid)
	}
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.MediaType) == "" || strings.TrimSpace(a.ObjectBucket) == "" || strings.TrimSpace(a.ObjectKey) == "" {
		return fmt.Errorf("%w: Artifact object evidence is incomplete", ErrAgentArtifactInvalid)
	}
	if a.SizeBytes == 0 || a.SizeBytes > AgentArtifactMaxBodyBytesV1 {
		return fmt.Errorf("%w: Artifact size is invalid", ErrAgentArtifactInvalid)
	}
	_, err := canonicalArtifactMetadataV1(a.Metadata)
	return err
}

func canonicalArtifactMetadataV1(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if len(raw) > AgentArtifactMaxMetadataV1 {
		return nil, fmt.Errorf("%w: metadata exceeds %d bytes", ErrAgentArtifactInvalid, AgentArtifactMaxMetadataV1)
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("%w: metadata must be one JSON object", ErrAgentArtifactInvalid)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: metadata contains trailing values", ErrAgentArtifactInvalid)
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical) > AgentArtifactMaxMetadataV1 {
		return nil, fmt.Errorf("%w: metadata cannot be canonicalized", ErrAgentArtifactInvalid)
	}
	return canonical, nil
}

func safeArtifactPathV1(value string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(digest[:16])
}
