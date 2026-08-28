package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JekYUlll/Dipole/internal/backfill/memorylineage"
)

func main() {
	reviewPath := flag.String("review-receipt", "", "rollout review receipt JSON path")
	evidencePath := flag.String("evidence", "", "shared-environment deployment evidence JSON path")
	receiptOut := flag.String("receipt-out", "", "deployment evidence receipt output path")
	flag.Parse()
	if err := run(*reviewPath, *evidencePath, *receiptOut); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(reviewPath, evidencePath, receiptOut string) error {
	if strings.TrimSpace(reviewPath) == "" || strings.TrimSpace(evidencePath) == "" || strings.TrimSpace(receiptOut) == "" {
		return fmt.Errorf("deployment evidence requires -review-receipt, -evidence and -receipt-out")
	}
	reviewData, err := os.ReadFile(reviewPath)
	if err != nil {
		return fmt.Errorf("read rollout review receipt: %w", err)
	}
	review, err := memorylineage.ParseRolloutReviewReceipt(reviewData)
	if err != nil {
		return err
	}
	evidenceData, err := os.ReadFile(evidencePath)
	if err != nil {
		return fmt.Errorf("read deployment evidence: %w", err)
	}
	evidence, err := memorylineage.ParseDeploymentEvidence(evidenceData)
	if err != nil {
		return err
	}
	receipt, err := memorylineage.BuildDeploymentEvidenceReceipt(review, evidence)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode deployment evidence receipt: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(receiptOut, encoded, 0o600); err != nil {
		return fmt.Errorf("write deployment evidence receipt: %w", err)
	}
	return nil
}
