package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JekYUlll/Dipole/internal/config"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	platformstorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	artifactreconcile "github.com/JekYUlll/Dipole/internal/reconcile/artifact"
	agentmysql "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

func main() {
	minimumAge := flag.Duration("minimum-age", artifactreconcile.MinimumSafeAgeV1, "minimum object age before orphan candidacy")
	maxExamples := flag.Int("max-examples", 100, "maximum object evidence examples")
	flag.Parse()
	if err := config.Load(); err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := platformmysql.InitMySQL(); err != nil {
		fatal(fmt.Errorf("initialize MySQL: %w", err))
	}
	defer platformmysql.SQLDB.Close()
	storageCfg := config.StorageConfig()
	source, err := platformstorage.NewAgentArtifactObjectSourceV1(ctx, platformstorage.AgentArtifactAuditConfigV1{
		Endpoint: storageCfg.ArtifactEndpoint, AccessKey: storageCfg.ArtifactAuditAccessKey,
		SecretKey: storageCfg.ArtifactAuditSecretKey, UseSSL: storageCfg.ArtifactUseSSL, Bucket: storageCfg.ArtifactBucket,
		RuntimeAccessKey: storageCfg.ArtifactAccessKey,
	})
	if err != nil {
		fatal(err)
	}
	metadata, err := agentmysql.NewAgentArtifactRepository(generated.New(platformmysql.SQLDB))
	if err != nil {
		fatal(err)
	}
	reconciler, err := artifactreconcile.New(source, metadata, artifactreconcile.ConfigV1{
		Bucket: storageCfg.ArtifactBucket, Prefix: "agent-artifacts/v1/", MinimumAge: *minimumAge, MaxExamples: *maxExamples,
	})
	if err != nil {
		fatal(err)
	}
	report, err := reconciler.Run(ctx)
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatal(fmt.Errorf("encode Artifact reconcile report: %w", err))
	}
	if !report.Consistent {
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
