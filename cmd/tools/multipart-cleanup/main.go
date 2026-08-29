package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/JekYUlll/Dipole/internal/config"
	storageops "github.com/JekYUlll/Dipole/internal/operations/storage"
)

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
	flag.Parse()
	if *olderThan <= 0 || (*execute && !*confirm) {
		fmt.Fprintln(os.Stderr, "older-than must be positive; --execute requires --confirm")
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
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if report.Failed > 0 {
		os.Exit(1)
	}
}
