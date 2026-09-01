package corefile

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
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
	FileSHA256   string    `json:"file_sha256,omitempty"`
	ChunkSize    int64     `json:"chunk_size"`
	TotalParts   int       `json:"total_parts"`
	CreatedAt    time.Time `json:"created_at"`
}

type multipartUploadSessionStore interface {
	Create(ctx context.Context, session *multipartUploadSession, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*multipartUploadSession, error)
	SaveCompleted(ctx context.Context, sessionID, uploaderUUID string, file *model.UploadedFile, ttl time.Duration) error
	GetCompleted(ctx context.Context, sessionID string) (*model.UploadedFile, string, error)
	SavePart(ctx context.Context, sessionID string, part *platformStorage.UploadedPart, ttl time.Duration) error
	ListParts(ctx context.Context, sessionID string) ([]platformStorage.MultipartCompletePart, error)
	Delete(ctx context.Context, sessionID string) error
}

// multipartPartPresence is optional so older session-store implementations can
// keep serving uploads while retry observation is rolled out.
type multipartPartPresence interface {
	HasPart(ctx context.Context, sessionID string, partNumber int) (bool, error)
}

type redisMultipartUploadSessionStore struct{}

type storedMultipartPart struct {
	ETag string `json:"etag"`
	Size int64  `json:"size"`
}

type storedMultipartCompletion struct {
	UploaderUUID string              `json:"uploader_uuid"`
	File         *model.UploadedFile `json:"file"`
}

func newMultipartUploadSessionStore() multipartUploadSessionStore {
	return &redisMultipartUploadSessionStore{}
}

func (s *redisMultipartUploadSessionStore) Create(ctx context.Context, session *multipartUploadSession, ttl time.Duration) error {
	if !platformCache.Available() {
		return fmt.Errorf("redis is not initialized")
	}
	if session == nil {
		return fmt.Errorf("multipart session is required")
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal multipart session: %w", err)
	}

	if err := platformCache.RunTransaction(ctx, func(pipe redis.Pipeliner) {
		pipe.Set(ctx, multipartSessionMetaKey(session.SessionID), payload, ttl)
		pipe.Del(ctx, multipartSessionPartsKey(session.SessionID))
	}); err != nil {
		return fmt.Errorf("store multipart session: %w", err)
	}
	return nil
}

func (s *redisMultipartUploadSessionStore) Get(ctx context.Context, sessionID string) (*multipartUploadSession, error) {
	if !platformCache.Available() {
		return nil, fmt.Errorf("redis is not initialized")
	}

	raw, err := platformCache.GetBytes(ctx, multipartSessionMetaKey(sessionID))
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

func (s *redisMultipartUploadSessionStore) SaveCompleted(ctx context.Context, sessionID, uploaderUUID string, file *model.UploadedFile, ttl time.Duration) error {
	if !platformCache.Available() {
		return fmt.Errorf("redis is not initialized")
	}
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(uploaderUUID) == "" || file == nil {
		return fmt.Errorf("multipart completion is invalid")
	}
	if err := platformCache.SetJSON(ctx, multipartSessionCompletedKey(sessionID), storedMultipartCompletion{UploaderUUID: strings.TrimSpace(uploaderUUID), File: file}, ttl); err != nil {
		return fmt.Errorf("store multipart completion: %w", err)
	}
	return nil
}

func (s *redisMultipartUploadSessionStore) GetCompleted(ctx context.Context, sessionID string) (*model.UploadedFile, string, error) {
	if !platformCache.Available() {
		return nil, "", fmt.Errorf("redis is not initialized")
	}
	raw, err := platformCache.GetBytes(ctx, multipartSessionCompletedKey(sessionID))
	if err != nil {
		if err == redis.Nil {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("get multipart completion: %w", err)
	}
	var stored storedMultipartCompletion
	if err := json.Unmarshal(raw, &stored); err != nil || stored.File == nil || strings.TrimSpace(stored.UploaderUUID) == "" {
		return nil, "", fmt.Errorf("unmarshal multipart completion: %w", err)
	}
	return stored.File, stored.UploaderUUID, nil
}

func (s *redisMultipartUploadSessionStore) SavePart(ctx context.Context, sessionID string, part *platformStorage.UploadedPart, ttl time.Duration) error {
	if !platformCache.Available() {
		return fmt.Errorf("redis is not initialized")
	}
	if part == nil {
		return fmt.Errorf("multipart part is required")
	}
	if part.PartNumber <= 0 || strings.TrimSpace(part.ETag) == "" || part.Size <= 0 {
		return fmt.Errorf("multipart part metadata is invalid")
	}
	partPayload, err := json.Marshal(storedMultipartPart{
		ETag: strings.TrimSpace(part.ETag),
		Size: part.Size,
	})
	if err != nil {
		return fmt.Errorf("marshal multipart part: %w", err)
	}

	if err := platformCache.RunTransaction(ctx, func(pipe redis.Pipeliner) {
		pipe.HSet(ctx, multipartSessionPartsKey(sessionID), strconv.Itoa(part.PartNumber), partPayload)
		pipe.Expire(ctx, multipartSessionMetaKey(sessionID), ttl)
		pipe.Expire(ctx, multipartSessionPartsKey(sessionID), ttl)
	}); err != nil {
		return fmt.Errorf("save multipart part: %w", err)
	}
	return nil
}

func (s *redisMultipartUploadSessionStore) HasPart(ctx context.Context, sessionID string, partNumber int) (bool, error) {
	if !platformCache.Available() {
		return false, fmt.Errorf("redis is not initialized")
	}
	if strings.TrimSpace(sessionID) == "" || partNumber <= 0 {
		return false, fmt.Errorf("multipart part identity is invalid")
	}
	present, err := platformCache.RDB.HExists(ctx, multipartSessionPartsKey(sessionID), strconv.Itoa(partNumber)).Result()
	if err != nil {
		return false, fmt.Errorf("check multipart part: %w", err)
	}
	return present, nil
}

func (s *redisMultipartUploadSessionStore) ListParts(ctx context.Context, sessionID string) ([]platformStorage.MultipartCompletePart, error) {
	if !platformCache.Available() {
		return nil, fmt.Errorf("redis is not initialized")
	}

	values, err := platformCache.HashGetAll(ctx, multipartSessionPartsKey(sessionID))
	if err != nil {
		return nil, fmt.Errorf("list multipart parts: %w", err)
	}
	parts := make([]platformStorage.MultipartCompletePart, 0, len(values))
	for key, rawPart := range values {
		partNumber, err := strconv.Atoi(key)
		if err != nil {
			return nil, fmt.Errorf("parse multipart part number: %w", err)
		}
		var stored storedMultipartPart
		if err := json.Unmarshal([]byte(rawPart), &stored); err != nil {
			// Preserve read compatibility for sessions created before size metadata.
			stored.ETag = strings.TrimSpace(rawPart)
		}
		parts = append(parts, platformStorage.MultipartCompletePart{
			PartNumber: partNumber,
			ETag:       strings.TrimSpace(stored.ETag),
			Size:       stored.Size,
		})
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	return parts, nil
}

func (s *redisMultipartUploadSessionStore) Delete(ctx context.Context, sessionID string) error {
	if !platformCache.Available() {
		return nil
	}
	if err := platformCache.Delete(ctx, multipartSessionMetaKey(sessionID), multipartSessionPartsKey(sessionID)); err != nil {
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

func multipartSessionCompletedKey(sessionID string) string {
	return "file:multipart:" + strings.TrimSpace(sessionID) + ":completed"
}
