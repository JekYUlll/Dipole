package agentmysql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

type AgentArtifactRepository struct{ queries generated.Querier }

var _ application.AgentArtifactStoreV1 = (*AgentArtifactRepository)(nil)

func NewAgentArtifactRepository(queries generated.Querier) (*AgentArtifactRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent Artifact queries are required")
	}
	return &AgentArtifactRepository{queries: queries}, nil
}

func (r *AgentArtifactRepository) CreateAgentArtifact(ctx context.Context, artifact application.AgentArtifactV1) (bool, error) {
	if err := artifact.Validate(); err != nil {
		return false, err
	}
	rows, err := r.queries.InsertAgentArtifact(ctx, generated.InsertAgentArtifactParams{
		ArtifactUuid: artifact.ArtifactUUID, SchemaVersion: artifact.SchemaVersion,
		TaskUuid: artifact.TaskUUID, RunUuid: artifact.RunUUID, ArtifactType: artifact.ArtifactType,
		Version: artifact.Version, Title: artifact.Title, MediaType: artifact.MediaType,
		ObjectBucket: artifact.ObjectBucket, ObjectKey: artifact.ObjectKey,
		ContentSha256: artifact.ContentSHA256, SizeBytes: artifact.SizeBytes, MetadataJson: artifact.Metadata,
	})
	if err != nil {
		return false, fmt.Errorf("create Agent Artifact: %w", err)
	}
	return rows > 0, nil
}

func (r *AgentArtifactRepository) GetAgentArtifact(ctx context.Context, artifactUUID string) (*application.AgentArtifactV1, error) {
	row, err := r.queries.GetAgentArtifact(ctx, artifactUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Artifact: %w", err)
	}
	return mapAgentArtifactV1(row), nil
}

func (r *AgentArtifactRepository) GetAgentArtifactByTaskTypeVersion(ctx context.Context, taskUUID, artifactType string, version uint32) (*application.AgentArtifactV1, error) {
	row, err := r.queries.GetAgentArtifactByTaskTypeVersion(ctx, generated.GetAgentArtifactByTaskTypeVersionParams{
		TaskUuid: taskUUID, ArtifactType: artifactType, Version: version,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent Artifact version: %w", err)
	}
	return mapAgentArtifactV1(row), nil
}

func (r *AgentArtifactRepository) ExistsByObjectKey(ctx context.Context, bucket, objectKey string) (bool, error) {
	exists, err := r.queries.AgentArtifactExistsByObjectKey(ctx, generated.AgentArtifactExistsByObjectKeyParams{
		ObjectBucket: bucket,
		ObjectKey:    objectKey,
	})
	if err != nil {
		return false, fmt.Errorf("lookup Agent Artifact object metadata: %w", err)
	}
	return exists, nil
}

func mapAgentArtifactV1(row generated.AgentArtifact) *application.AgentArtifactV1 {
	metadata := row.MetadataJson
	var compact bytes.Buffer
	if json.Compact(&compact, row.MetadataJson) == nil {
		metadata = compact.Bytes()
	}
	return &application.AgentArtifactV1{
		ArtifactUUID: row.ArtifactUuid, SchemaVersion: row.SchemaVersion,
		TaskUUID: row.TaskUuid, RunUUID: row.RunUuid, ArtifactType: row.ArtifactType, Version: row.Version,
		Title: row.Title, MediaType: row.MediaType, ObjectBucket: row.ObjectBucket, ObjectKey: row.ObjectKey,
		ContentSHA256: row.ContentSha256, SizeBytes: row.SizeBytes, Metadata: metadata, CreatedAt: row.CreatedAt,
	}
}
