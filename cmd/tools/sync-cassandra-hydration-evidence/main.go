package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/operations/sync/evidence"
)

func main() {
	evidencePath := flag.String("evidence", "", "Sync hydration evidence JSON")
	policyPath := flag.String("policy", "", "Sync hydration policy JSON")
	flag.Parse()
	if *evidencePath == "" || *policyPath == "" {
		fail(fmt.Errorf("-evidence and -policy are required"))
	}
	evidenceData, err := os.ReadFile(*evidencePath)
	if err != nil {
		fail(err)
	}
	policyData, err := os.ReadFile(*policyPath)
	if err != nil {
		fail(err)
	}
	evidence, err := synchhydration.ParseEvidence(evidenceData)
	if err != nil {
		fail(err)
	}
	policy, err := synchhydration.ParsePolicy(policyData)
	if err != nil {
		fail(err)
	}
	report, err := synchhydration.Evaluate(evidence, policy)
	if err != nil {
		fail(err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(encoded))
	if report.Decision != "eligible" {
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Sync Cassandra hydration evidence is invalid:", err)
	os.Exit(1)
}
