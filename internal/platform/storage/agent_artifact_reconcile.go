package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	artifactreconcile "github.com/JekYUlll/Dipole/internal/reconcile/artifact"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type AgentArtifactAuditConfigV1 struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	Bucket           string
	RuntimeAccessKey string
}

type AgentArtifactObjectSourceV1 struct {
	client *minio.Client
	bucket string
}

var _ artifactreconcile.ObjectSourceV1 = (*AgentArtifactObjectSourceV1)(nil)

func NewAgentArtifactObjectSourceV1(ctx context.Context, cfg AgentArtifactAuditConfigV1) (*AgentArtifactObjectSourceV1, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" || accessKey == strings.TrimSpace(cfg.RuntimeAccessKey) {
		return nil, errors.New("Agent Artifact audit storage configuration is incomplete")
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		return nil, fmt.Errorf("create Agent Artifact audit client: %w", err)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(checkCtx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check Agent Artifact audit bucket: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("Agent Artifact audit bucket %s does not exist", bucket)
	}
	return &AgentArtifactObjectSourceV1{client: client, bucket: bucket}, nil
}

func (s *AgentArtifactObjectSourceV1) Walk(ctx context.Context, prefix string, visit func(artifactreconcile.ObjectEvidenceV1) error) error {
	if s == nil || s.client == nil || s.bucket == "" || strings.TrimSpace(prefix) != "agent-artifacts/v1/" || visit == nil {
		return errors.New("invalid Agent Artifact audit walk")
	}
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		if err := visit(artifactreconcile.ObjectEvidenceV1{
			Key: object.Key, SizeBytes: object.Size, LastModified: object.LastModified, ETag: object.ETag,
		}); err != nil {
			return err
		}
	}
	return ctx.Err()
}
