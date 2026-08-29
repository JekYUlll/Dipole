package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	artifactcleanup "github.com/JekYUlll/Dipole/internal/cleanup/artifact"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	platformstorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	artifactreconcile "github.com/JekYUlll/Dipole/internal/reconcile/artifact"
	"github.com/JekYUlll/Dipole/internal/store"
)

func main() {
	action := flag.String("action", "", "maintenance action: authorize or evaluate")
	input := flag.String("input", "", "input reconcile report or authorization JSON")
	objectKey := flag.String("object-key", "", "exact orphan candidate object key")
	proposalID := flag.String("proposal-id", "", "approved maintenance proposal ID")
	proposerID := flag.String("proposer-id", "", "proposal author identity")
	approverIDs := flag.String("approver-ids", "", "two comma-separated approver identities")
	executorID := flag.String("executor-id", "", "independent evaluator identity")
	grantVersion := flag.String("grant-version", "", "maintenance grant version")
	lifetime := flag.Duration("lifetime", 10*time.Minute, "authorization lifetime, maximum 15m")
	flag.Parse()
	if strings.TrimSpace(*input) == "" {
		fatal(errors.New("-input is required"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	switch strings.TrimSpace(*action) {
	case "authorize":
		var report artifactreconcile.ReportV1
		readJSON(*input, &report)
		now := time.Now().UTC()
		authorization, err := artifactcleanup.NewAuthorizationV1(report, *objectKey, artifactcleanup.AuthorizationRolesV1{
			ProposalID: *proposalID, ProposerID: *proposerID, ApproverIDs: strings.Split(*approverIDs, ","),
			ExecutorID: *executorID, MaintenanceGrantVersion: *grantVersion,
		}, now, now.Add(*lifetime))
		if err != nil {
			fatal(err)
		}
		writeJSON(authorization)
	case "evaluate":
		var authorization artifactcleanup.AuthorizationV1
		readJSON(*input, &authorization)
		if err := authorization.Verify(); err != nil {
			fatal(err)
		}
		if err := config.Load(); err != nil {
			fatal(fmt.Errorf("load config: %w", err))
		}
		if err := store.InitMySQL(); err != nil {
			fatal(fmt.Errorf("initialize MySQL: %w", err))
		}
		defer store.SQLDB.Close()
		cfg := config.StorageConfig()
		inspector, err := platformstorage.NewAgentArtifactMaintenanceInspectorV1(platformstorage.AgentArtifactMaintenanceConfigV1{
			Endpoint: cfg.ArtifactEndpoint, AccessKey: cfg.ArtifactMaintenanceAccessKey, SecretKey: cfg.ArtifactMaintenanceSecretKey,
			UseSSL: cfg.ArtifactUseSSL, Bucket: cfg.ArtifactBucket, RuntimeAccessKey: cfg.ArtifactAccessKey, AuditAccessKey: cfg.ArtifactAuditAccessKey,
		})
		if err != nil {
			fatal(err)
		}
		metadata, err := repository.NewAgentArtifactRepository(generated.New(store.SQLDB))
		if err != nil {
			fatal(err)
		}
		evaluator, err := artifactcleanup.NewDryRunEvaluatorV1(metadata, inspector, time.Now)
		if err != nil {
			fatal(err)
		}
		receipt, err := evaluator.Evaluate(ctx, authorization)
		if err != nil {
			fatal(err)
		}
		writeJSON(receipt)
		if receipt.Outcome != artifactcleanup.OutcomeWouldDeleteV1 {
			os.Exit(2)
		}
	default:
		fatal(errors.New("-action must be authorize or evaluate"))
	}
}

func readJSON(path string, target any) {
	body, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		fatal(fmt.Errorf("decode %s: trailing JSON value", path))
	}
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
