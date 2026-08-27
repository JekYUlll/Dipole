package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/minio/minio-go/v7"
)

type agentArtifactObjectClient interface {
	Get(context.Context, string, string) ([]byte, bool, error)
	Put(context.Context, string, string, string, []byte) error
}

type AgentArtifactBlobStore struct {
	client agentArtifactObjectClient
	bucket string
}

var _ application.AgentArtifactBlobStoreV1 = (*AgentArtifactBlobStore)(nil)

func NewAgentArtifactBlobStore(storage ObjectStorage) (*AgentArtifactBlobStore, error) {
	uploader, ok := storage.(*MinIOUploader)
	if !ok || uploader == nil || uploader.client == nil || strings.TrimSpace(uploader.bucket) == "" {
		return nil, errors.New("Agent Artifact storage requires initialized MinIO")
	}
	return newAgentArtifactBlobStoreV1(&minioAgentArtifactClientV1{client: uploader.client}, uploader.bucket), nil
}

func newAgentArtifactBlobStoreV1(client agentArtifactObjectClient, bucket string) *AgentArtifactBlobStore {
	return &AgentArtifactBlobStore{client: client, bucket: strings.TrimSpace(bucket)}
}

func (s *AgentArtifactBlobStore) PutImmutable(ctx context.Context, objectKey, mediaType string, body []byte, contentSHA256 string) (application.AgentArtifactBlobReceiptV1, error) {
	objectKey = strings.TrimSpace(objectKey)
	mediaType = strings.TrimSpace(mediaType)
	if s == nil || s.client == nil || s.bucket == "" || !strings.HasPrefix(objectKey, "agent-artifacts/v1/") || mediaType == "" {
		return application.AgentArtifactBlobReceiptV1{}, errors.New("invalid Agent Artifact object request")
	}
	digest := sha256.Sum256(body)
	if len(body) == 0 || len(body) > application.AgentArtifactMaxBodyBytesV1 || hex.EncodeToString(digest[:]) != strings.TrimSpace(contentSHA256) {
		return application.AgentArtifactBlobReceiptV1{}, errors.New("Agent Artifact body does not match content evidence")
	}
	existing, found, err := s.client.Get(ctx, s.bucket, objectKey)
	if err != nil {
		return application.AgentArtifactBlobReceiptV1{}, err
	}
	if found {
		if !bytes.Equal(existing, body) {
			return application.AgentArtifactBlobReceiptV1{}, fmt.Errorf("%w: content-addressed object differs", application.ErrAgentArtifactConflict)
		}
		return application.AgentArtifactBlobReceiptV1{Bucket: s.bucket, ObjectKey: objectKey}, nil
	}
	if err := s.client.Put(ctx, s.bucket, objectKey, mediaType, body); err != nil {
		return application.AgentArtifactBlobReceiptV1{}, err
	}
	return application.AgentArtifactBlobReceiptV1{Bucket: s.bucket, ObjectKey: objectKey}, nil
}

func (s *AgentArtifactBlobStore) Open(ctx context.Context, receipt application.AgentArtifactBlobReceiptV1) (io.ReadCloser, error) {
	if s == nil || s.client == nil || strings.TrimSpace(receipt.Bucket) != s.bucket || !strings.HasPrefix(strings.TrimSpace(receipt.ObjectKey), "agent-artifacts/v1/") {
		return nil, errors.New("Agent Artifact receipt does not match configured storage")
	}
	body, found, err := s.client.Get(ctx, s.bucket, receipt.ObjectKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("Agent Artifact object is unavailable")
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

type minioAgentArtifactClientV1 struct{ client *minio.Client }

func (c *minioAgentArtifactClientV1) Get(ctx context.Context, bucket, objectKey string) ([]byte, bool, error) {
	object, err := c.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, false, err
	}
	defer object.Close()
	body, err := io.ReadAll(io.LimitReader(object, application.AgentArtifactMaxBodyBytesV1+1))
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" {
			return nil, false, nil
		}
		return nil, false, err
	}
	return body, true, nil
}

func (c *minioAgentArtifactClientV1) Put(ctx context.Context, bucket, objectKey, mediaType string, body []byte) error {
	_, err := c.client.PutObject(ctx, bucket, objectKey, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{
		ContentType: mediaType,
		UserMetadata: map[string]string{"dipole-content-sha256": func() string {
			digest := sha256.Sum256(body)
			return hex.EncodeToString(digest[:])
		}()},
	})
	return err
}
