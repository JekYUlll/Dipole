package storageops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type MultipartReconciliationCandidate struct {
	SessionID string `json:"session_id,omitempty"`
	ObjectKey string `json:"object_key"`
	UploadID  string `json:"upload_id"`
	Reason    string `json:"reason"`
}

type MultipartReconciliationReport struct {
	Bucket           string                             `json:"bucket"`
	Prefix           string                             `json:"prefix"`
	RedisKeysScanned int                                `json:"redis_keys_scanned"`
	MinIOUploadsSeen int                                `json:"minio_uploads_seen"`
	MissingRedis     int                                `json:"missing_redis"`
	MissingMinIO     int                                `json:"missing_minio"`
	Complete         bool                               `json:"complete"`
	Errors           []string                           `json:"errors,omitempty"`
	Candidates       []MultipartReconciliationCandidate `json:"candidates"`
}

type multipartSessionReference struct {
	SessionID string `json:"-"`
	ObjectKey string `json:"object_key"`
	UploadID  string `json:"upload_id"`
}

// RunMultipartReconciliation compares unfinished MinIO uploads with Redis
// session metadata. It is intentionally read-only; deletion stays separate.
func RunMultipartReconciliation(ctx context.Context, client MultipartClient, redisClient *redis.Client, bucket, prefix string, maxRedisKeys int64) MultipartReconciliationReport {
	report := MultipartReconciliationReport{Bucket: bucket, Prefix: prefix, Complete: true, Candidates: make([]MultipartReconciliationCandidate, 0)}
	if client == nil || redisClient == nil {
		report.Complete = false
		report.Errors = append(report.Errors, "MinIO and Redis clients are required")
		return report
	}
	if maxRedisKeys <= 0 {
		maxRedisKeys = 10000
	}
	redisRefs, complete, err := scanMultipartSessionReferences(ctx, redisClient, maxRedisKeys)
	if err != nil {
		report.Complete = false
		report.Errors = append(report.Errors, fmt.Sprintf("scan Redis sessions: %v", err))
	} else if !complete {
		report.Complete = false
	}
	report.RedisKeysScanned = len(redisRefs)
	minioRefs := make(map[string]multipartSessionReference)
	for upload := range client.ListIncompleteUploads(ctx, bucket, prefix, true) {
		if upload.Err != nil {
			report.Complete = false
			report.Errors = append(report.Errors, fmt.Sprintf("list MinIO upload: %v", upload.Err))
			continue
		}
		report.MinIOUploadsSeen++
		ref := multipartSessionReference{ObjectKey: upload.Key, UploadID: upload.UploadID}
		minioRefs[multipartReferenceKey(ref.ObjectKey, ref.UploadID)] = ref
	}
	for key, ref := range minioRefs {
		if _, ok := redisRefs[key]; ok {
			continue
		}
		report.MissingRedis++
		report.Candidates = append(report.Candidates, MultipartReconciliationCandidate{ObjectKey: ref.ObjectKey, UploadID: ref.UploadID, Reason: "minio_upload_without_redis_session"})
	}
	for key, ref := range redisRefs {
		if _, ok := minioRefs[key]; ok {
			continue
		}
		report.MissingMinIO++
		report.Candidates = append(report.Candidates, MultipartReconciliationCandidate{SessionID: ref.SessionID, ObjectKey: ref.ObjectKey, UploadID: ref.UploadID, Reason: "redis_session_without_minio_upload"})
	}
	return report
}

func scanMultipartSessionReferences(ctx context.Context, client *redis.Client, maxKeys int64) (map[string]multipartSessionReference, bool, error) {
	refs := make(map[string]multipartSessionReference)
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, multipartSessionKeyPrefix+"*:meta", 100).Result()
		if err != nil {
			return refs, false, err
		}
		for _, key := range keys {
			if int64(len(refs)) >= maxKeys {
				return refs, false, nil
			}
			id, ok := multipartSessionID(key, ":meta")
			if !ok {
				continue
			}
			raw, err := client.Get(ctx, key).Bytes()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				return refs, false, fmt.Errorf("read %s: %w", key, err)
			}
			var session multipartSessionReference
			if err := json.Unmarshal(raw, &session); err != nil || strings.TrimSpace(session.ObjectKey) == "" || strings.TrimSpace(session.UploadID) == "" {
				return refs, false, fmt.Errorf("invalid multipart session %s", key)
			}
			session.SessionID = id
			refs[multipartReferenceKey(session.ObjectKey, session.UploadID)] = session
		}
		cursor = next
		if cursor == 0 {
			return refs, true, nil
		}
	}
}

func multipartReferenceKey(objectKey, uploadID string) string {
	return strings.TrimSpace(objectKey) + "\x00" + strings.TrimSpace(uploadID)
}
