package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/backfill/memorylineage"
)

func main() {
	jobName := flag.String("job", "", "backfill job name bound to the approval")
	manifestPath := flag.String("manifest", "", "fixed backfill manifest JSON path")
	approvalPath := flag.String("approval", "", "independent approval JSON path")
	inputPath := flag.String("input", "", "review input JSON path")
	receiptOut := flag.String("receipt-out", "", "rollout review receipt output path")
	nowValue := flag.String("now", "", "optional RFC3339 time for deterministic review")
	flag.Parse()
	if err := run(*jobName, *manifestPath, *approvalPath, *inputPath, *receiptOut, *nowValue); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(jobName, manifestPath, approvalPath, inputPath, receiptOut, nowValue string) error {
	if strings.TrimSpace(jobName) == "" || strings.TrimSpace(manifestPath) == "" || strings.TrimSpace(approvalPath) == "" || strings.TrimSpace(inputPath) == "" || strings.TrimSpace(receiptOut) == "" {
		return fmt.Errorf("rollout review requires -job, -manifest, -approval, -input and -receipt-out")
	}
	manifest, err := memorylineage.ParseManifestFile(manifestPath)
	if err != nil {
		return err
	}
	approval, err := memorylineage.ParseApprovalFile(approvalPath, manifest, jobName)
	if err != nil {
		return err
	}
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read rollout review input: %w", err)
	}
	input, err := memorylineage.ParseRolloutReviewInput(inputData)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(nowValue) != "" {
		now, err = time.Parse(time.RFC3339Nano, nowValue)
		if err != nil {
			return fmt.Errorf("parse rollout review time: %w", err)
		}
	}
	// The review is read-only and has no dependency on the backfill runtime.
	receipt, err := memorylineage.BuildRolloutReviewReceipt(manifest, approval, input, now)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rollout review receipt: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(receiptOut, encoded, 0o600); err != nil {
		return fmt.Errorf("write rollout review receipt: %w", err)
	}
	return nil
}
