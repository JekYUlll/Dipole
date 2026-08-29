package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/evidence/cassandrareadrollout"
)

func main() {
	evidencePath := flag.String("evidence", "", "path to Cassandra read rollout evidence JSON")
	policyPath := flag.String("policy", "", "path to Cassandra read rollout policy JSON")
	flag.Parse()
	if *evidencePath == "" || *policyPath == "" {
		fmt.Fprintln(os.Stderr, "cassandra read rollout evidence requires -evidence=<path> and -policy=<path>")
		os.Exit(1)
	}
	evidenceData, err := os.ReadFile(*evidencePath)
	if err != nil {
		fail(err)
	}
	policyData, err := os.ReadFile(*policyPath)
	if err != nil {
		fail(err)
	}
	evidence, err := cassandrareadrollout.ParseEvidence(evidenceData)
	if err != nil {
		fail(err)
	}
	policy, err := cassandrareadrollout.ParsePolicy(policyData)
	if err != nil {
		fail(err)
	}
	report, err := cassandrareadrollout.Evaluate(evidence, policy)
	if err != nil {
		fail(err)
	}
	if err := printJSON(report); err != nil {
		fail(err)
	}
	if report.Decision != "eligible" {
		os.Exit(2)
	}
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cassandra read rollout evidence is invalid:", err)
	os.Exit(1)
}
