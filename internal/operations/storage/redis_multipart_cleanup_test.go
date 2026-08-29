package storageops

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRunRedisMultipartCleanupDryRunReportsMissingTTLAndOrphans(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	ctx := context.Background()
	client.Set(ctx, "file:multipart:active:meta", "{}", time.Minute)
	client.HSet(ctx, "file:multipart:active:parts", "1", "etag")
	client.Set(ctx, "file:multipart:no-ttl:meta", "{}", 0)
	client.HSet(ctx, "file:multipart:orphan:parts", "1", "etag")

	report := RunRedisMultipartCleanup(ctx, client, 10, 100, false)
	if !report.Complete || report.MetaScanned != 2 || report.PartsScanned != 2 || report.MissingTTL != 1 || report.OrphanParts != 1 || report.DeletedParts != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if mini.Exists("file:multipart:orphan:parts") == false {
		t.Fatal("dry-run deleted orphan parts")
	}
}

func TestRunRedisMultipartCleanupExecuteDeletesOnlyOrphanParts(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	ctx := context.Background()
	client.HSet(ctx, "file:multipart:orphan:parts", "1", "etag")
	client.Set(ctx, "file:multipart:active:meta", "{}", time.Minute)
	client.HSet(ctx, "file:multipart:active:parts", "1", "etag")

	report := RunRedisMultipartCleanup(ctx, client, 10, 100, true)
	if report.Failed != 0 || report.DeletedParts != 1 || report.OrphanParts != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if mini.Exists("file:multipart:orphan:parts") {
		t.Fatal("execute did not delete orphan parts")
	}
	if !mini.Exists("file:multipart:active:parts") {
		t.Fatal("execute deleted active parts")
	}
}
