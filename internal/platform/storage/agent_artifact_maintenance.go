package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	artifactcleanup "github.com/JekYUlll/Dipole/internal/cleanup/artifact"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type AgentArtifactMaintenanceConfigV1 struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	Bucket           string
	RuntimeAccessKey string
	AuditAccessKey   string
}

type AgentArtifactMaintenanceInspectorV1 struct {
	client *minio.Client
	bucket string
}

var _ artifactcleanup.ObjectInspectorV1 = (*AgentArtifactMaintenanceInspectorV1)(nil)

func NewAgentArtifactMaintenanceInspectorV1(cfg AgentArtifactMaintenanceConfigV1) (*AgentArtifactMaintenanceInspectorV1, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" ||
		accessKey == strings.TrimSpace(cfg.RuntimeAccessKey) || accessKey == strings.TrimSpace(cfg.AuditAccessKey) {
		return nil, errors.New("Agent Artifact maintenance inspection configuration is incomplete")
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, fmt.Errorf("create Agent Artifact maintenance inspection client: %w", err)
	}
	return &AgentArtifactMaintenanceInspectorV1{client: client, bucket: bucket}, nil
}

func (s *AgentArtifactMaintenanceInspectorV1) Inspect(ctx context.Context, bucket, objectKey string) (artifactcleanup.ObjectStateV1, error) {
	if s == nil || s.client == nil || strings.TrimSpace(bucket) != s.bucket || !strings.HasPrefix(strings.TrimSpace(objectKey), "agent-artifacts/v1/") {
		return artifactcleanup.ObjectStateV1{}, errors.New("invalid Agent Artifact maintenance inspection")
	}
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" {
			return artifactcleanup.ObjectStateV1{}, nil
		}
		return artifactcleanup.ObjectStateV1{}, err
	}
	return artifactcleanup.ObjectStateV1{Found: true, ETag: strings.Trim(info.ETag, `"`), SizeBytes: info.Size, LastModified: info.LastModified.UTC()}, nil
}
