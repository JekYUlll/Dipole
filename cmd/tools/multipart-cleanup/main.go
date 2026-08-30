package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	storageops "github.com/JekYUlll/Dipole/internal/operations/storage"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

type cleanupOutput struct {
	storageops.MultipartCleanupReport
	Redis          *storageops.RedisMultipartCleanupReport   `json:"redis,omitempty"`
	Reconciliation *storageops.MultipartReconciliationReport `json:"reconciliation,omitempty"`
}

type minioMultipartClient struct {
	client *minio.Client
	core   minio.Core
}

func (c minioMultipartClient) ListIncompleteUploads(ctx context.Context, bucket, prefix string, recursive bool) <-chan minio.ObjectMultipartInfo {
	return c.client.ListIncompleteUploads(ctx, bucket, prefix, recursive)
}

func (c minioMultipartClient) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	return c.core.AbortMultipartUpload(ctx, bucket, objectKey, uploadID)
}

func main() {
	prefix := flag.String("prefix", "message-files/", "object prefix to scan")
	olderThan := flag.Duration("older-than", 24*time.Hour, "minimum age of an incomplete upload")
	execute := flag.Bool("execute", false, "abort eligible uploads; omitted means dry-run")
	confirm := flag.Bool("confirm", false, "confirm that aborting eligible uploads is intentional")
	redisOrphans := flag.Bool("redis-orphans", false, "scan and report Redis multipart session anomalies")
	reconcile := flag.Bool("reconcile", false, "read-only compare MinIO incomplete uploads with Redis sessions")
	reconcileFailOnDrift := flag.Bool("reconcile-fail-on-drift", false, "exit 3 when reconciliation finds cross-store drift")
	metricsOutput := flag.String("metrics-output", "", "write reconciliation gauges in Prometheus textfile format")
	redisScanCount := flag.Int64("redis-scan-count", 100, "Redis SCAN batch size")
	redisMaxKeys := flag.Int64("redis-max-keys", 10000, "maximum Redis keys to inspect per key family")
	flag.Parse()
	if *olderThan <= 0 || (*execute && !*confirm) || *redisScanCount <= 0 || *redisMaxKeys <= 0 || (*reconcileFailOnDrift && !*reconcile) || (*metricsOutput != "" && !*reconcile) {
		fmt.Fprintln(os.Stderr, "older-than, redis-scan-count and redis-max-keys must be positive; --execute requires --confirm; --reconcile-fail-on-drift and --metrics-output require --reconcile")
		os.Exit(2)
	}
	if err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := config.StorageConfig()
	if !cfg.Enabled || strings.ToLower(strings.TrimSpace(cfg.Provider)) != "minio" {
		fmt.Fprintln(os.Stderr, "MinIO storage must be enabled")
		os.Exit(1)
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	report := storageops.RunMultipartCleanup(ctx, minioMultipartClient{client: client, core: minio.Core{Client: client}}, cfg.Bucket, storageops.NormalizePrefix(*prefix), time.Now().UTC().Add(-*olderThan), *execute)
	output := cleanupOutput{MultipartCleanupReport: report}
	var redisClient *redis.Client
	if *redisOrphans || *reconcile {
		var redisErr error
		redisClient, redisErr = platformCache.NewRedisClient(config.RedisConfig())
		if redisErr != nil {
			fmt.Fprintln(os.Stderr, redisErr)
			os.Exit(1)
		}
		defer redisClient.Close()
		if *redisOrphans {
			redisReport := storageops.RunRedisMultipartCleanup(ctx, redisClient, *redisScanCount, *redisMaxKeys, *execute && *confirm)
			output.Redis = &redisReport
			if !redisReport.Complete || redisReport.Failed > 0 {
				report.Failed++
			}
		}
		if *reconcile {
			reconciliation := storageops.RunMultipartReconciliation(ctx, minioMultipartClient{client: client, core: minio.Core{Client: client}}, redisClient, cfg.Bucket, storageops.NormalizePrefix(*prefix), *redisMaxKeys)
			output.Reconciliation = &reconciliation
			if !reconciliation.Complete {
				report.Failed++
			}
		}
	}
	output.MultipartCleanupReport = report
	if *metricsOutput != "" {
		if err := writeMultipartReconciliationMetrics(*metricsOutput, output.Reconciliation, time.Now().UTC()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if report.Failed > 0 {
		os.Exit(1)
	}
	if *reconcileFailOnDrift && reconciliationHasDrift(output.Reconciliation) {
		os.Exit(3)
	}
}

func reconciliationHasDrift(report *storageops.MultipartReconciliationReport) bool {
	return report != nil && (report.MissingRedis > 0 || report.MissingMinIO > 0)
}

func writeMultipartReconciliationMetrics(path string, report *storageops.MultipartReconciliationReport, capturedAt time.Time) error {
	if report == nil {
		return fmt.Errorf("multipart reconciliation report is required for metrics")
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	complete := 0
	if report.Complete {
		complete = 1
	}
	drift := 0
	if reconciliationHasDrift(report) {
		drift = 1
	}
	var body strings.Builder
	body.WriteString("# HELP dipole_multipart_reconciliation_complete Whether the last reconciliation scan completed without scan errors.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_complete gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_complete %d\n", complete)
	body.WriteString("# HELP dipole_multipart_reconciliation_drift Whether the last complete scan found cross-store drift.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_drift gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_drift %d\n", drift)
	body.WriteString("# HELP dipole_multipart_reconciliation_redis_keys_scanned Redis multipart metadata keys scanned.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_redis_keys_scanned gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_redis_keys_scanned %d\n", report.RedisKeysScanned)
	body.WriteString("# HELP dipole_multipart_reconciliation_minio_uploads_seen Incomplete MinIO multipart uploads seen.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_minio_uploads_seen gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_minio_uploads_seen %d\n", report.MinIOUploadsSeen)
	body.WriteString("# HELP dipole_multipart_reconciliation_missing_redis MinIO uploads without matching Redis session metadata.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_missing_redis gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_missing_redis %d\n", report.MissingRedis)
	body.WriteString("# HELP dipole_multipart_reconciliation_missing_minio Redis sessions without matching MinIO upload.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_missing_minio gauge\n")
	fmt.Fprintf(&body, "dipole_multipart_reconciliation_missing_minio %d\n", report.MissingMinIO)
	body.WriteString("# HELP dipole_multipart_reconciliation_last_run_timestamp_seconds Unix timestamp of the last reconciliation scan.\n")
	body.WriteString("# TYPE dipole_multipart_reconciliation_last_run_timestamp_seconds gauge\n")
	body.WriteString("dipole_multipart_reconciliation_last_run_timestamp_seconds ")
	body.WriteString(strconv.FormatInt(capturedAt.Unix(), 10))
	body.WriteByte('\n')

	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create metrics directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".multipart-reconciliation-*.prom.tmp")
	if err != nil {
		return fmt.Errorf("create metrics temp file: %w", err)
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o640); err != nil {
		return fmt.Errorf("set metrics file mode: %w", err)
	}
	if _, err := temporary.WriteString(body.String()); err != nil {
		return fmt.Errorf("write metrics file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync metrics file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close metrics file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish metrics file: %w", err)
	}
	keep = true
	return nil
}
