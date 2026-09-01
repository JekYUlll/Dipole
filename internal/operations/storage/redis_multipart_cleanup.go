package storageops

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

const multipartSessionKeyPrefix = "file:multipart:"

type RedisMultipartCleanupCandidate struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
	Reason    string `json:"reason"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type RedisMultipartCleanupReport struct {
	Execute      bool                             `json:"execute"`
	ScanCount    int64                            `json:"scan_count"`
	MaxKeys      int64                            `json:"max_keys"`
	MetaScanned  int                              `json:"meta_scanned"`
	MissingTTL   int                              `json:"missing_ttl"`
	PartsScanned int                              `json:"parts_scanned"`
	OrphanParts  int                              `json:"orphan_parts"`
	DeletedParts int                              `json:"deleted_parts"`
	Failed       int                              `json:"failed"`
	Complete     bool                             `json:"complete"`
	Candidates   []RedisMultipartCleanupCandidate `json:"candidates"`
	Errors       []string                         `json:"errors,omitempty"`
}

// RunRedisMultipartCleanup reports missing-expiry metadata and part hashes
// whose session metadata has already expired. Only orphan part hashes may be
// deleted, and only when execute is explicitly enabled.
func RunRedisMultipartCleanup(ctx context.Context, client *redis.Client, scanCount, maxKeys int64, execute bool) RedisMultipartCleanupReport {
	report := RedisMultipartCleanupReport{
		Execute: execute, ScanCount: scanCount, MaxKeys: maxKeys,
		Complete: true, Candidates: make([]RedisMultipartCleanupCandidate, 0),
	}
	if client == nil {
		report.Complete = false
		report.Errors = append(report.Errors, "redis client is nil")
		return report
	}
	if scanCount <= 0 {
		scanCount = 100
		report.ScanCount = scanCount
	}
	if maxKeys <= 0 {
		maxKeys = 10000
		report.MaxKeys = maxKeys
	}

	metaKeys, metaComplete, err := scanMultipartKeys(ctx, client, "*:meta", scanCount, maxKeys)
	if err != nil {
		report.Complete = false
		report.Errors = append(report.Errors, fmt.Sprintf("scan metadata keys: %v", err))
	} else if !metaComplete {
		report.Complete = false
	}
	for _, key := range metaKeys {
		report.MetaScanned++
		sessionID, ok := multipartSessionID(key, ":meta")
		if !ok {
			continue
		}
		ttl, ttlErr := client.TTL(ctx, key).Result()
		if ttlErr != nil {
			report.Complete = false
			report.Errors = append(report.Errors, fmt.Sprintf("read TTL for %s: %v", key, ttlErr))
			continue
		}
		if ttl == -1 {
			report.MissingTTL++
			report.Candidates = append(report.Candidates, RedisMultipartCleanupCandidate{
				SessionID: sessionID, Key: key, Reason: "metadata_missing_ttl", Status: "review",
			})
		}
	}

	partKeys, partsComplete, err := scanMultipartKeys(ctx, client, "*:parts", scanCount, maxKeys)
	if err != nil {
		report.Complete = false
		report.Errors = append(report.Errors, fmt.Sprintf("scan part keys: %v", err))
	} else if !partsComplete {
		report.Complete = false
	}
	for _, key := range partKeys {
		report.PartsScanned++
		sessionID, ok := multipartSessionID(key, ":parts")
		if !ok {
			continue
		}
		metaKey := multipartSessionKeyPrefix + sessionID + ":meta"
		exists, existsErr := client.Exists(ctx, metaKey).Result()
		if existsErr != nil {
			report.Complete = false
			report.Errors = append(report.Errors, fmt.Sprintf("check metadata for %s: %v", key, existsErr))
			continue
		}
		if exists > 0 {
			continue
		}
		report.OrphanParts++
		candidate := RedisMultipartCleanupCandidate{
			SessionID: sessionID, Key: key, Reason: "parts_without_metadata", Status: "eligible",
		}
		if execute {
			deleted, deleteErr := client.Del(ctx, key).Result()
			if deleteErr != nil {
				report.Failed++
				candidate.Status = "failed"
				candidate.Error = deleteErr.Error()
			} else if deleted > 0 {
				report.DeletedParts++
				candidate.Status = "deleted"
			} else {
				candidate.Status = "already_deleted"
			}
		}
		report.Candidates = append(report.Candidates, candidate)
	}
	return report
}

func scanMultipartKeys(ctx context.Context, client *redis.Client, suffix string, scanCount, maxKeys int64) ([]string, bool, error) {
	keys := make([]string, 0)
	var cursor uint64
	for {
		batch, next, err := client.Scan(ctx, cursor, multipartSessionKeyPrefix+"*"+suffix, scanCount).Result()
		if err != nil {
			return keys, false, err
		}
		remaining := maxKeys - int64(len(keys))
		if remaining < int64(len(batch)) {
			batch = batch[:max(0, int(remaining))]
		}
		keys = append(keys, batch...)
		if int64(len(keys)) >= maxKeys {
			return keys, false, nil
		}
		cursor = next
		if cursor == 0 {
			return keys, true, nil
		}
	}
}

func multipartSessionID(key, suffix string) (string, bool) {
	if !strings.HasPrefix(key, multipartSessionKeyPrefix) || !strings.HasSuffix(key, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(key, multipartSessionKeyPrefix), suffix)
	if id == "" || strings.Contains(id, ":") {
		return "", false
	}
	return id, true
}
