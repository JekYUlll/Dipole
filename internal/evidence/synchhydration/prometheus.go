package synchhydration

import (
	"bytes"
	"fmt"
	"io"
	"math"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	routeMetricName    = "dipole_sync_hydration_route_total"
	durationMetricName = "dipole_sync_hydration_route_duration_seconds"
)

// PrometheusSnapshotMetadata binds an untrusted metrics snapshot to its operator-supplied window identity.
type PrometheusSnapshotMetadata struct {
	Service            string
	DeploymentRevision string
	Mode               string
	WindowStart        string
	WindowEnd          string
}

// EvidenceFromPrometheus converts runtime collector output into the existing low-sensitivity evidence contract.
func EvidenceFromPrometheus(data []byte, metadata PrometheusSnapshotMetadata) (Evidence, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Evidence{}, fmt.Errorf("Prometheus snapshot is empty")
	}
	if metadata.Service == "" || metadata.DeploymentRevision == "" || metadata.Mode == "" || metadata.WindowStart == "" || metadata.WindowEnd == "" {
		return Evidence{}, fmt.Errorf("Prometheus snapshot metadata is incomplete")
	}
	var routeCounts Counts
	var hitHistogram *dto.Histogram
	decoder := expfmt.NewDecoder(bytes.NewReader(data), expfmt.NewFormat(expfmt.TypeTextPlain))
	for {
		var family dto.MetricFamily
		if err := decoder.Decode(&family); err != nil {
			if err == io.EOF {
				break
			}
			return Evidence{}, fmt.Errorf("decode Prometheus snapshot: %w", err)
		}
		switch family.GetName() {
		case routeMetricName:
			if err := collectRouteCounts(&routeCounts, family.GetMetric()); err != nil {
				return Evidence{}, err
			}
		case durationMetricName:
			for _, metric := range family.GetMetric() {
				if labelValue(metric.GetLabel(), "outcome") != "hit" {
					continue
				}
				if hitHistogram != nil {
					return Evidence{}, fmt.Errorf("Prometheus snapshot contains duplicate hit histograms")
				}
				hitHistogram = metric.GetHistogram()
			}
		}
	}
	if routeCounts.Total == 0 {
		return Evidence{}, fmt.Errorf("Prometheus snapshot has no hydration requests")
	}
	p95, err := histogramP95Micros(hitHistogram)
	if err != nil {
		return Evidence{}, err
	}
	evidence := Evidence{SchemaVersion: "dipole.sync-cassandra-hydration-evidence.v1", Service: metadata.Service, DeploymentRevision: metadata.DeploymentRevision, Mode: metadata.Mode, WindowStart: metadata.WindowStart, WindowEnd: metadata.WindowEnd, Requests: routeCounts, Latency: Latency{CassandraP95Micros: p95}}
	if err := validateEvidence(evidence); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func collectRouteCounts(counts *Counts, metrics []*dto.Metric) error {
	for _, metric := range metrics {
		value := metric.GetCounter().GetValue()
		outcome := labelValue(metric.GetLabel(), "outcome")
		if outcome == "" || metric.Counter == nil || value < 0 || math.Trunc(value) != value {
			return fmt.Errorf("Prometheus route metric is invalid")
		}
		count := uint64(value)
		switch outcome {
		case "hit":
			counts.CassandraHit += count
		case "fallback":
			counts.MySQLFallback += count
		case "error", "cancelled":
			counts.Error += count
		default:
			return fmt.Errorf("Prometheus route outcome is unsupported")
		}
		counts.Total += count
	}
	return nil
}

func histogramP95Micros(histogram *dto.Histogram) (uint64, error) {
	if histogram == nil || histogram.GetSampleCount() == 0 {
		return 0, fmt.Errorf("Prometheus snapshot has no hit latency histogram")
	}
	rank := (histogram.GetSampleCount()*95 + 99) / 100
	for _, bucket := range histogram.GetBucket() {
		if bucket.GetCumulativeCount() < rank {
			continue
		}
		upper := bucket.GetUpperBound()
		if math.IsInf(upper, 0) || upper < 0 || upper > float64(math.MaxUint64)/1e6 {
			return 0, fmt.Errorf("Prometheus hit latency p95 bucket is invalid")
		}
		return uint64(math.Ceil(upper * 1e6)), nil
	}
	return 0, fmt.Errorf("Prometheus hit latency histogram has no finite p95 bucket")
}

func labelValue(labels []*dto.LabelPair, name string) string {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}
