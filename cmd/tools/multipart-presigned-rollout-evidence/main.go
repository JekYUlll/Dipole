package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/operations/storage/presignedrollout"
)

func main() {
	evidencePath := flag.String("evidence", "", "path to Multipart presigned rollout evidence JSON")
	policyPath := flag.String("policy", "", "path to Multipart presigned rollout policy JSON")
	printPolicySHA := flag.Bool("print-policy-sha", false, "print the canonical SHA-256 for -policy and exit")
	flag.Parse()
	if *policyPath == "" || (!*printPolicySHA && *evidencePath == "") {
		fail(fmt.Errorf("Multipart presigned rollout evidence requires -evidence=<path> and -policy=<path>, or -policy=<path> -print-policy-sha"))
	}
	policyData, err := os.ReadFile(*policyPath)
	if err != nil {
		fail(err)
	}
	policy, err := presignedrollout.ParsePolicy(policyData)
	if err != nil {
		fail(err)
	}
	if *printPolicySHA {
		fmt.Println(presignedrollout.PolicySHA256(policy))
		return
	}
	evidenceData, err := os.ReadFile(*evidencePath)
	if err != nil {
		fail(err)
	}
	evidence, err := presignedrollout.ParseEvidence(evidenceData)
	if err != nil {
		fail(err)
	}
	report, err := presignedrollout.Evaluate(evidence, policy)
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
	fmt.Fprintln(os.Stderr, "Multipart presigned rollout evidence is invalid:", err)
	os.Exit(1)
}
