package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/store"
)

type multipartUploadSession struct {
	SessionID    string    `json:"session_id"`
	UploaderUUID string    `json:"uploader_uuid"`
	Bucket       string    `json:"bucket"`
	ObjectKey    string    `json:"object_key"`
	UploadID     string    `json:"upload_id"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	ContentType  string    `json:"content_type"`
	ChunkSize    int64     `json:"chunk_size"`
	TotalParts   int       `json:"total_parts"`
	CreatedAt    time.Time `json:"created_at"`
}

type multipartUploadSessionStore interface {
	Create(ctx context.Context, session *multipartUploadSession, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*multipartUploadSession, error)
	SavePart(ctx context.Context, sessionID string, part *platformStorage.UploadedPart, ttl time.Duration) error
	ListParts(ctx context.Context, sessionID string) ([]platformStorage.MultipartCompletePart, error)
	Delete(ctx context.Context, sessionID string) error
}

type redisMultipartUploadSessionStore struct{}

func newMultipartUploadSessionStore() multipartUploadSessionStore {
	return &redisMultipartUploadSessionStore{}
}

func (s *redisMultipartUploadSessionStore) Create(ctx context.Context, session *multipartUploadSession, ttl time.Duration) error {
	if store.RDB == nil {
		return fmt.Errorf("redis is not initialized")
	}
	if session == nil {
		return fmt.Errorf("multipart session is required")
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal multipart session: %w", err)
	}

	pipe := store.RDB.TxPipeline()
	pipe.Set(ctx, multipartSessionMetaKey(session.SessionID), payload, ttl)
	pipe.Del(ctx, multipartSessionPartsKey(session.SessionID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store multipart session: %w", err)
	}
	return nil
}

func (s *redisMultipartUploadSessionStore) Get(ctx context.Context, sessionID string) (*multipartUploadSession, error) {
	if store.RDB == nil {
		return nil, fmt.Errorf("redis is not initialized")
	}

	raw, err := store.RDB.Get(ctx, multipartSessionMetaKey(sessionID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get multipart session: %w", err)
	}

	var session multipartUploadSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("unmarshal multipart session: %w", err)
	}
	return &session, nil
}

func (s *redisMultipartUploadSessionStore) SavePart(ctx context.Context, sessionID string, part *platformStorage.UploadedPart, ttl time.Duration) error {
	if store.RDB == nil {
		return fmt.Errorf("redis is not initialized")
	}
	if part == nil {
		return fmt.Errorf("multipart part is required")
	}

	pipe := store.RDB.TxPipeline()
	pipe.HSet(ctx, multipartSessionPartsKey(sessionID), strconv.Itoa(part.PartNumber), strings.TrimSpace(part.ETag))
	pipe.Expire(ctx, multipartSessionMetaKey(sessionID), ttl)
	pipe.Expire(ctx, multipartSessionPartsKey(sessionID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save multipart part: %w", err)
	}
	return nil
}

func (s *redisMultipartUploadSessionStore) ListParts(ctx context.Context, sessionID string) ([]platformStorage.MultipartCompletePart, error) {
	if store.RDB == nil {
		return nil, fmt.Errorf("redis is not initialized")
	}

	values, err := store.RDB.HGetAll(ctx, multipartSessionPartsKey(sessionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list multipart parts: %w", err)
	}
	parts := make([]platformStorage.MultipartCompletePart, 0, len(values))
	for key, etag := range values {
		partNumber, err := strconv.Atoi(key)
		if err != nil {
			return nil, fmt.Errorf("parse multipart part number: %w", err)
		}
		parts = append(parts, platformStorage.MultipartCompletePart{
			PartNumber: partNumber,
			ETag:       strings.TrimSpace(etag),
		})
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	return parts, nil
}

func (s *redisMultipartUploadSessionStore) Delete(ctx context.Context, sessionID string) error {
	if store.RDB == nil {
		return nil
	}
	if err := store.RDB.Del(ctx, multipartSessionMetaKey(sessionID), multipartSessionPartsKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete multipart session: %w", err)
	}
	return nil
}

func multipartSessionMetaKey(sessionID string) string {
	return "file:multipart:" + strings.TrimSpace(sessionID) + ":meta"
}

func multipartSessionPartsKey(sessionID string) string {
	return "file:multipart:" + strings.TrimSpace(sessionID) + ":parts"
}
