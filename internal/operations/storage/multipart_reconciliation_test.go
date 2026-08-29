package storageops

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

func TestRunMultipartReconciliationReportsCrossStoreDrift(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	ctx := context.Background()
	active, _ := json.Marshal(map[string]string{"object_key": "message-files/active", "upload_id": "upload-active"})
	orphan, _ := json.Marshal(map[string]string{"object_key": "message-files/missing", "upload_id": "upload-missing"})
	client.Set(ctx, "file:multipart:active:meta", active, 0)
	client.Set(ctx, "file:multipart:missing:meta", orphan, 0)
	minioClient := &multipartClientStub{uploads: []minio.ObjectMultipartInfo{
		{Key: "message-files/active", UploadID: "upload-active"},
		{Key: "message-files/no-session", UploadID: "upload-no-session"},
	}}

	report := RunMultipartReconciliation(ctx, minioClient, client, "files", "message-files/", 100)
	if !report.Complete || report.RedisKeysScanned != 2 || report.MinIOUploadsSeen != 2 || report.MissingRedis != 1 || report.MissingMinIO != 1 {
		t.Fatalf("unexpected reconciliation report: %+v", report)
	}
	if len(report.Candidates) != 2 {
		t.Fatalf("unexpected candidates: %+v", report.Candidates)
	}
}

func TestRunMultipartReconciliationIsReadOnly(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer client.Close()
	ctx := context.Background()
	client.Set(ctx, "file:multipart:missing:meta", `{"object_key":"k","upload_id":"u"}`, 0)
	_ = RunMultipartReconciliation(ctx, &multipartClientStub{uploads: []minio.ObjectMultipartInfo{{Key: "other", UploadID: "other"}}}, client, "files", "", 100)
	if !mini.Exists("file:multipart:missing:meta") {
		t.Fatal("reconciliation mutated Redis")
	}
}
