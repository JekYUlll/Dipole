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

	"github.com/JekYUlll/Dipole/internal/config"
	agentops "github.com/JekYUlll/Dipole/internal/operations/agent"
	"github.com/JekYUlll/Dipole/internal/operations/agent/memorylineage"
)

func main() {
	jobName := flag.String("job", "memory-lineage-v1", "durable backfill job name")
	ownerID := flag.String("owner", defaultOwnerID(), "lease owner identity")
	operatorID := flag.String("operator", "", "reviewed operator identity for execute mode")
	approverID := flag.String("approver", "", "independent approver identity for execute mode")
	batchSize := flag.Int("batch-size", 100, "plans read before checkpoint advance")
	leaseSeconds := flag.Int("lease-seconds", 60, "owner lease duration renewed after each batch")
	execute := flag.Bool("execute", false, "execute only with matching manifest and approval")
	manifestPath := flag.String("manifest", "", "fixed manifest JSON path for execute mode")
	manifestOut := flag.String("manifest-out", "", "manifest output path for dry-run mode")
	approvalPath := flag.String("approval", "", "reviewed approval JSON path for execute mode")
	receiptOut := flag.String("receipt-out", "", "low-sensitive receipt output path")
	flag.Parse()

	if err := run(*jobName, *ownerID, *operatorID, *approverID, *batchSize, *leaseSeconds, *execute, *manifestPath, *manifestOut, *approvalPath, *receiptOut); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(jobName, ownerID, operatorID, approverID string, batchSize, leaseSeconds int, execute bool, manifestPath, manifestOut, approvalPath, receiptOut string) error {
	if strings.TrimSpace(jobName) == "" || strings.TrimSpace(ownerID) == "" || batchSize <= 0 || leaseSeconds < 1 {
		return fmt.Errorf("invalid Memory lineage backfill options")
	}
	if execute {
		if strings.TrimSpace(operatorID) == "" || strings.TrimSpace(approverID) == "" || strings.TrimSpace(operatorID) == strings.TrimSpace(approverID) || strings.TrimSpace(manifestPath) == "" || strings.TrimSpace(approvalPath) == "" || strings.TrimSpace(receiptOut) == "" {
			return fmt.Errorf("execute mode requires distinct -operator and -approver, -manifest, -approval and -receipt-out")
		}
	} else if strings.TrimSpace(manifestOut) == "" {
		return fmt.Errorf("dry-run requires -manifest-out")
	}
	if err := config.Load(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !execute {
		highWatermark, err := agentops.ReadMemoryLineageBackfillHighWatermark(ctx)
		if err != nil {
			return err
		}
		manifest, err := memorylineage.NewManifest(highWatermark, uint32(batchSize))
		if err != nil {
			return err
		}
		return writeJSON(manifestOut, manifest)
	}
	manifest, err := memorylineage.ParseManifestFile(manifestPath)
	if err != nil {
		return err
	}
	approval, err := memorylineage.ParseApprovalFile(approvalPath, manifest, jobName)
	if err != nil {
		return err
	}
	if approval.OperatorID != strings.TrimSpace(operatorID) || approval.ApproverID != strings.TrimSpace(approverID) {
		return fmt.Errorf("Memory lineage backfill approval identity mismatch")
	}
	result, err := agentops.RunMemoryLineageBackfill(ctx, agentops.MemoryLineageBackfillOptions{
		JobName: jobName, OwnerID: ownerID, BatchSize: int(manifest.BatchSize), LeaseDuration: time.Duration(leaseSeconds) * time.Second,
	})
	if err != nil {
		return err
	}
	receipt, err := memorylineage.BuildReceipt(manifest, result)
	if err != nil {
		return err
	}
	return writeJSON(receiptOut, receipt)
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Memory lineage backfill output: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write Memory lineage backfill output: %w", err)
	}
	return nil
}

func defaultOwnerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
