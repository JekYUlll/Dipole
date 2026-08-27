package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentArtifactPolicyStoreV1 interface {
	GetTask(context.Context, string) (*application.AgentTaskV1, error)
	GetRun(context.Context, string) (*application.AgentRunV1, error)
}

type PersistentAgentArtifactServiceV1 struct {
	policies  agentArtifactPolicyStoreV1
	artifacts application.AgentArtifactStoreV1
	blobs     application.AgentArtifactBlobStoreV1
}

func NewPersistentAgentArtifactServiceV1(policies agentArtifactPolicyStoreV1, artifacts application.AgentArtifactStoreV1, blobs application.AgentArtifactBlobStoreV1) (*PersistentAgentArtifactServiceV1, error) {
	if policies == nil || artifacts == nil || blobs == nil {
		return nil, errors.New("Agent Artifact policy, metadata, and blob stores are required")
	}
	return &PersistentAgentArtifactServiceV1{policies: policies, artifacts: artifacts, blobs: blobs}, nil
}

func (s *PersistentAgentArtifactServiceV1) Create(ctx context.Context, input application.AgentArtifactCreateV1) (*application.AgentArtifactV1, error) {
	candidate, err := application.NewAgentArtifactV1(input)
	if err != nil {
		return nil, err
	}
	task, run, err := s.loadBinding(ctx, candidate.TaskUUID, candidate.RunUUID)
	if err != nil {
		return nil, err
	}
	if task.TenantID != strings.TrimSpace(input.TenantID) {
		return nil, fmt.Errorf("%w: Artifact tenant does not match the Task", application.ErrAgentArtifactDenied)
	}
	existing, err := s.artifacts.GetAgentArtifactByTaskTypeVersion(ctx, candidate.TaskUUID, candidate.ArtifactType, candidate.Version)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !sameAgentArtifactCandidateV1(existing, candidate) {
			return nil, fmt.Errorf("%w: Task/type/version is already bound", application.ErrAgentArtifactConflict)
		}
		if err := s.verifyStoredBody(ctx, existing, input.Content); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if run.Status != application.AgentRunStatusRunning || run.RuntimeID != "dipole-agent" || run.Mode != "shadow" {
		return nil, fmt.Errorf("%w: Artifact creation requires the active authenticated shadow Run", application.ErrAgentArtifactDenied)
	}
	receipt, err := s.blobs.PutImmutable(ctx, candidate.ObjectKey, candidate.MediaType, input.Content, candidate.ContentSHA256)
	if err != nil {
		return nil, fmt.Errorf("store Agent Artifact body: %w", err)
	}
	candidate.ObjectBucket = strings.TrimSpace(receipt.Bucket)
	if strings.TrimSpace(receipt.ObjectKey) != candidate.ObjectKey {
		return nil, fmt.Errorf("%w: blob receipt conflicts with content address", application.ErrAgentArtifactConflict)
	}
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	created, err := s.artifacts.CreateAgentArtifact(ctx, *candidate)
	if err != nil {
		return nil, err
	}
	if created {
		persisted, loadErr := s.artifacts.GetAgentArtifact(ctx, candidate.ArtifactUUID)
		if loadErr != nil {
			return nil, loadErr
		}
		if persisted == nil || !sameAgentArtifactV1(persisted, candidate) {
			return nil, fmt.Errorf("%w: persisted Artifact differs", application.ErrAgentArtifactConflict)
		}
		return persisted, nil
	}
	existing, err = s.artifacts.GetAgentArtifact(ctx, candidate.ArtifactUUID)
	if err != nil {
		return nil, err
	}
	if existing == nil || !sameAgentArtifactV1(existing, candidate) {
		return nil, fmt.Errorf("%w: immutable Artifact replay differs", application.ErrAgentArtifactConflict)
	}
	return existing, nil
}

func (s *PersistentAgentArtifactServiceV1) GetForPrincipal(ctx context.Context, principalUUID, artifactUUID string) (*application.AgentArtifactV1, []byte, error) {
	artifact, err := s.artifacts.GetAgentArtifact(ctx, strings.TrimSpace(artifactUUID))
	if err != nil {
		return nil, nil, err
	}
	if artifact == nil {
		return nil, nil, fmt.Errorf("%w: Artifact is unavailable", application.ErrAgentArtifactDenied)
	}
	task, err := s.policies.GetTask(ctx, artifact.TaskUUID)
	if err != nil {
		return nil, nil, err
	}
	if task == nil || strings.TrimSpace(principalUUID) == "" || task.PrincipalUUID != strings.TrimSpace(principalUUID) {
		return nil, nil, fmt.Errorf("%w: Artifact is unavailable", application.ErrAgentArtifactDenied)
	}
	body, err := s.readVerifiedBody(ctx, artifact)
	if err != nil {
		return nil, nil, err
	}
	return artifact, body, nil
}

func (s *PersistentAgentArtifactServiceV1) verifyStoredBody(ctx context.Context, artifact *application.AgentArtifactV1, expected []byte) error {
	body, err := s.readVerifiedBody(ctx, artifact)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, expected) {
		return fmt.Errorf("%w: replay body differs", application.ErrAgentArtifactConflict)
	}
	return nil
}

func (s *PersistentAgentArtifactServiceV1) readVerifiedBody(ctx context.Context, artifact *application.AgentArtifactV1) ([]byte, error) {
	reader, err := s.blobs.Open(ctx, application.AgentArtifactBlobReceiptV1{Bucket: artifact.ObjectBucket, ObjectKey: artifact.ObjectKey})
	if err != nil {
		return nil, fmt.Errorf("open Agent Artifact body: %w", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, application.AgentArtifactMaxBodyBytesV1+1))
	if err != nil {
		return nil, fmt.Errorf("read Agent Artifact body: %w", err)
	}
	digest := sha256.Sum256(body)
	if uint64(len(body)) != artifact.SizeBytes || hex.EncodeToString(digest[:]) != artifact.ContentSHA256 {
		return nil, fmt.Errorf("%w: stored body evidence differs", application.ErrAgentArtifactConflict)
	}
	return body, nil
}

func (s *PersistentAgentArtifactServiceV1) loadBinding(ctx context.Context, taskUUID, runUUID string) (*application.AgentTaskV1, *application.AgentRunV1, error) {
	task, err := s.policies.GetTask(ctx, taskUUID)
	if err != nil {
		return nil, nil, err
	}
	run, err := s.policies.GetRun(ctx, runUUID)
	if err != nil {
		return nil, nil, err
	}
	if task == nil || run == nil || run.TaskUUID != taskUUID {
		return nil, nil, fmt.Errorf("%w: Task and Run binding is unavailable", application.ErrAgentArtifactDenied)
	}
	return task, run, nil
}

func sameAgentArtifactCandidateV1(existing, candidate *application.AgentArtifactV1) bool {
	if existing == nil || candidate == nil {
		return existing == nil && candidate == nil
	}
	return existing.ArtifactUUID == candidate.ArtifactUUID && existing.TaskUUID == candidate.TaskUUID &&
		existing.RunUUID == candidate.RunUUID && existing.ArtifactType == candidate.ArtifactType &&
		existing.Version == candidate.Version && existing.Title == candidate.Title && existing.MediaType == candidate.MediaType &&
		existing.ObjectKey == candidate.ObjectKey &&
		existing.ContentSHA256 == candidate.ContentSHA256 && existing.SizeBytes == candidate.SizeBytes &&
		bytes.Equal(existing.Metadata, candidate.Metadata)
}

func sameAgentArtifactV1(left, right *application.AgentArtifactV1) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	l, r := *left, *right
	l.CreatedAt = time.Time{}
	r.CreatedAt = time.Time{}
	return reflect.DeepEqual(l, r)
}
