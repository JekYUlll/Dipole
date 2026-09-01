package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/JekYUlll/Dipole/internal/operations/cassandra/evidence"
)

func main() {
	startMetricsPath := flag.String("metrics-start", "", "Prometheus text exposition snapshot at window start")
	endMetricsPath := flag.String("metrics-end", "", "Prometheus text exposition snapshot at window end")
	service := flag.String("service", "message-service", "service name")
	revision := flag.String("revision", "", "deployed Message Service revision")
	percentage := flag.Int("percentage", -1, "configured Cassandra read percentage")
	windowStart := flag.String("window-start", "", "RFC3339 window start")
	windowEnd := flag.String("window-end", "", "RFC3339 window end")
	flag.Parse()
	if *startMetricsPath == "" || *endMetricsPath == "" || *revision == "" || *percentage < 0 || *windowStart == "" || *windowEnd == "" {
		fail(fmt.Errorf("-metrics-start, -metrics-end, -revision, -percentage, -window-start, and -window-end are required"))
	}
	startData, err := os.ReadFile(*startMetricsPath)
	if err != nil {
		fail(err)
	}
	endData, err := os.ReadFile(*endMetricsPath)
	if err != nil {
		fail(err)
	}
	window, err := cassandrareadrollout.EvidenceFromPrometheusWindow(startData, endData, cassandrareadrollout.PrometheusSnapshotMetadata{
		Service: *service, DeploymentRevision: *revision, ConfiguredReadPercentage: *percentage,
		WindowStart: *windowStart, WindowEnd: *windowEnd,
	})
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(window, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(data))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Cassandra read rollout snapshot is invalid:", err)
	os.Exit(1)
}
