package corefile

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

func TestRedisMultipartSessionTTLExpiresMetadataAndPartsTogether(t *testing.T) {
	server := miniredis.RunT(t)
	previousRedis := platformCache.RDB
	platformCache.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = platformCache.RDB.Close()
		platformCache.RDB = previousRedis
	})

	ctx := context.Background()
	store := &redisMultipartUploadSessionStore{}
	session := &multipartUploadSession{
		SessionID:    "session-ttl",
		UploaderUUID: "user-ttl",
		Bucket:       "files",
		ObjectKey:    "message-files/ttl.bin",
		UploadID:     "upload-ttl",
		FileName:     "ttl.bin",
		FileSize:     10,
		ContentType:  "application/octet-stream",
		ChunkSize:    5,
		TotalParts:   2,
		CreatedAt:    time.Now().UTC(),
	}

	if err := store.Create(ctx, session, 2*time.Second); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := store.SavePart(ctx, session.SessionID, &platformStorage.UploadedPart{
		PartNumber: 1,
		ETag:       "etag-1",
		Size:       5,
	}, 2*time.Second); err != nil {
		t.Fatalf("SavePart(): %v", err)
	}

	server.FastForward(1500 * time.Millisecond)
	if got, err := store.Get(ctx, session.SessionID); err != nil || got == nil {
		t.Fatalf("session expired before refreshed TTL: got=%+v err=%v", got, err)
	}
	if parts, err := store.ListParts(ctx, session.SessionID); err != nil || len(parts) != 1 {
		t.Fatalf("parts expired before refreshed TTL: parts=%+v err=%v", parts, err)
	}

	server.FastForward(600 * time.Millisecond)
	if got, err := store.Get(ctx, session.SessionID); err != nil || got != nil {
		t.Fatalf("session metadata survived TTL: got=%+v err=%v", got, err)
	}
	if parts, err := store.ListParts(ctx, session.SessionID); err != nil || len(parts) != 0 {
		t.Fatalf("parts survived TTL: parts=%+v err=%v", parts, err)
	}

	present, err := platformCache.Exists(ctx, multipartSessionMetaKey(session.SessionID))
	if err != nil || present {
		t.Fatalf("metadata key remains after TTL: present=%t err=%v", present, err)
	}
	present, err = platformCache.Exists(ctx, multipartSessionPartsKey(session.SessionID))
	if err != nil || present {
		t.Fatalf("parts key remains after TTL: present=%t err=%v", present, err)
	}
}

func TestRedisMultipartSessionCompletionUsesIndependentTTL(t *testing.T) {
	server := miniredis.RunT(t)
	previousRedis := platformCache.RDB
	platformCache.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = platformCache.RDB.Close()
		platformCache.RDB = previousRedis
	})

	ctx := context.Background()
	store := &redisMultipartUploadSessionStore{}
	file := &model.UploadedFile{UUID: "file-ttl", Bucket: "files", ObjectKey: "message-files/ttl.bin"}
	if err := store.SaveCompleted(ctx, "session-complete-ttl", "user-ttl", file, 2*time.Second); err != nil {
		t.Fatalf("SaveCompleted(): %v", err)
	}

	server.FastForward(2100 * time.Millisecond)
	got, uploader, err := store.GetCompleted(ctx, "session-complete-ttl")
	if err != nil || got != nil || uploader != "" {
		t.Fatalf("completion receipt survived TTL: file=%+v uploader=%q err=%v", got, uploader, err)
	}
}
