package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/evidence/agenttimelinerepair"
)

func main() {
	evidencePath := flag.String("evidence", "", "path to Agent Timeline repair rollout evidence JSON")
	policyPath := flag.String("policy", "", "path to Agent Timeline repair rollout policy JSON")
	flag.Parse()
	if *evidencePath == "" || *policyPath == "" {
		fail(fmt.Errorf("repair rollout evidence requires -evidence=<path> and -policy=<path>"))
	}
	evidenceData, err := os.ReadFile(*evidencePath)
	if err != nil {
		fail(err)
	}
	policyData, err := os.ReadFile(*policyPath)
	if err != nil {
		fail(err)
	}
	evidence, err := agenttimelinerepair.ParseEvidence(evidenceData)
	if err != nil {
		fail(err)
	}
	policy, err := agenttimelinerepair.ParsePolicy(policyData)
	if err != nil {
		fail(err)
	}
	report, err := agenttimelinerepair.Evaluate(evidence, policy)
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(data))
	if report.Decision != "eligible" {
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Agent Timeline repair rollout evidence is invalid:", err)
	os.Exit(1)
}
