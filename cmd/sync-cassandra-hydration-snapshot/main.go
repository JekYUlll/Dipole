package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/evidence/synchhydration"
)

func main() {
	metricsPath := flag.String("metrics", "", "Prometheus text exposition snapshot")
	service := flag.String("service", "", "service name")
	revision := flag.String("revision", "", "deployment revision")
	mode := flag.String("mode", "", "shadow or primary")
	windowStart := flag.String("window-start", "", "RFC3339 window start")
	windowEnd := flag.String("window-end", "", "RFC3339 window end")
	flag.Parse()
	if *metricsPath == "" {
		fail(fmt.Errorf("-metrics is required"))
	}
	data, err := os.ReadFile(*metricsPath)
	if err != nil {
		fail(err)
	}
	evidence, err := synchhydration.EvidenceFromPrometheus(data, synchhydration.PrometheusSnapshotMetadata{Service: *service, DeploymentRevision: *revision, Mode: *mode, WindowStart: *windowStart, WindowEnd: *windowEnd})
	if err != nil {
		fail(err)
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(encoded))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Sync Cassandra hydration snapshot is invalid:", err)
	os.Exit(1)
}
