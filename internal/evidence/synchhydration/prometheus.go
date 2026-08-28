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

// EvidenceFromPrometheus converts one cumulative collector output into evidence.
// Call EvidenceFromPrometheusWindow for a bounded rollout window.
func EvidenceFromPrometheus(data []byte, metadata PrometheusSnapshotMetadata) (Evidence, error) {
	snapshot, err := parsePrometheusSnapshot(data, true)
	if err != nil {
		return Evidence{}, err
	}
	return evidenceFromSnapshot(snapshot.routes, snapshot.hitHistogram, metadata)
}

// EvidenceFromPrometheusWindow converts two cumulative collector outputs into bounded-window evidence.
func EvidenceFromPrometheusWindow(startData, endData []byte, metadata PrometheusSnapshotMetadata) (Evidence, error) {
	start, err := parsePrometheusSnapshot(startData, false)
	if err != nil {
		return Evidence{}, fmt.Errorf("parse start snapshot: %w", err)
	}
	end, err := parsePrometheusSnapshot(endData, true)
	if err != nil {
		return Evidence{}, fmt.Errorf("parse end snapshot: %w", err)
	}
	delta, err := subtractCounts(end.routes, start.routes)
	if err != nil {
		return Evidence{}, err
	}
	hitHistogram, err := subtractHistograms(end.hitHistogram, start.hitHistogram)
	if err != nil {
		return Evidence{}, err
	}
	return evidenceFromSnapshot(delta, hitHistogram, metadata)
}

type prometheusSnapshot struct {
	routes       Counts
	hitHistogram *dto.Histogram
}

func parsePrometheusSnapshot(data []byte, requireRequests bool) (prometheusSnapshot, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return prometheusSnapshot{}, fmt.Errorf("Prometheus snapshot is empty")
	}
	var routeCounts Counts
	var hitHistogram *dto.Histogram
	routeFamilySeen := false
	durationFamilySeen := false
	decoder := expfmt.NewDecoder(bytes.NewReader(data), expfmt.NewFormat(expfmt.TypeTextPlain))
	for {
		var family dto.MetricFamily
		if err := decoder.Decode(&family); err != nil {
			if err == io.EOF {
				break
			}
			return prometheusSnapshot{}, fmt.Errorf("decode Prometheus snapshot: %w", err)
		}
		switch family.GetName() {
		case routeMetricName:
			if routeFamilySeen || family.GetType() != dto.MetricType_COUNTER {
				return prometheusSnapshot{}, fmt.Errorf("Prometheus route metric family is duplicated or has the wrong type")
			}
			routeFamilySeen = true
			if err := collectRouteCounts(&routeCounts, family.GetMetric()); err != nil {
				return prometheusSnapshot{}, err
			}
		case durationMetricName:
			if durationFamilySeen || family.GetType() != dto.MetricType_HISTOGRAM {
				return prometheusSnapshot{}, fmt.Errorf("Prometheus duration metric family is duplicated or has the wrong type")
			}
			durationFamilySeen = true
			for _, metric := range family.GetMetric() {
				outcome, err := exactOutcome(metric.GetLabel())
				if err != nil {
					return prometheusSnapshot{}, err
				}
				if metric.Histogram == nil || outcome != "hit" {
					return prometheusSnapshot{}, fmt.Errorf("Prometheus duration metric has an unsupported outcome")
				}
				if hitHistogram != nil {
					return prometheusSnapshot{}, fmt.Errorf("Prometheus snapshot contains duplicate hit histograms")
				}
				hitHistogram = metric.GetHistogram()
				if err := validateHistogram(hitHistogram); err != nil {
					return prometheusSnapshot{}, err
				}
			}
		}
	}
	if requireRequests && routeCounts.Total == 0 {
		return prometheusSnapshot{}, fmt.Errorf("Prometheus snapshot has no hydration requests")
	}
	return prometheusSnapshot{routes: routeCounts, hitHistogram: hitHistogram}, nil
}

func evidenceFromSnapshot(counts Counts, hitHistogram *dto.Histogram, metadata PrometheusSnapshotMetadata) (Evidence, error) {
	if metadata.Service == "" || metadata.DeploymentRevision == "" || metadata.Mode == "" || metadata.WindowStart == "" || metadata.WindowEnd == "" {
		return Evidence{}, fmt.Errorf("Prometheus snapshot metadata is incomplete")
	}
	p95, err := histogramP95Micros(hitHistogram)
	if err != nil {
		return Evidence{}, err
	}
	evidence := Evidence{SchemaVersion: "dipole.sync-cassandra-hydration-evidence.v1", Service: metadata.Service, DeploymentRevision: metadata.DeploymentRevision, Mode: metadata.Mode, WindowStart: metadata.WindowStart, WindowEnd: metadata.WindowEnd, Requests: counts, Latency: Latency{CassandraP95Micros: p95}}
	if err := validateEvidence(evidence); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func subtractCounts(end, start Counts) (Counts, error) {
	if end.Total < start.Total || end.CassandraHit < start.CassandraHit || end.MySQLFallback < start.MySQLFallback || end.Missing < start.Missing || end.Conflict < start.Conflict || end.Error < start.Error {
		return Counts{}, fmt.Errorf("Prometheus counters moved backwards")
	}
	return Counts{Total: end.Total - start.Total, CassandraHit: end.CassandraHit - start.CassandraHit, MySQLFallback: end.MySQLFallback - start.MySQLFallback, Missing: end.Missing - start.Missing, Conflict: end.Conflict - start.Conflict, Error: end.Error - start.Error}, nil
}

func subtractHistograms(end, start *dto.Histogram) (*dto.Histogram, error) {
	if end == nil || start == nil || end.GetSampleCount() < start.GetSampleCount() {
		return nil, fmt.Errorf("Prometheus hit latency histograms are missing or moved backwards")
	}
	if len(end.GetBucket()) != len(start.GetBucket()) {
		return nil, fmt.Errorf("Prometheus hit latency histogram buckets changed")
	}
	result := &dto.Histogram{SampleCount: uint64Ptr(end.GetSampleCount() - start.GetSampleCount()), SampleSum: float64Ptr(end.GetSampleSum() - start.GetSampleSum())}
	for i, bucket := range end.GetBucket() {
		previous := start.GetBucket()[i]
		if bucket.GetUpperBound() != previous.GetUpperBound() || bucket.GetCumulativeCount() < previous.GetCumulativeCount() {
			return nil, fmt.Errorf("Prometheus hit latency histogram buckets moved backwards or changed")
		}
		result.Bucket = append(result.Bucket, &dto.Bucket{UpperBound: float64Ptr(bucket.GetUpperBound()), CumulativeCount: uint64Ptr(bucket.GetCumulativeCount() - previous.GetCumulativeCount())})
	}
	return result, nil
}

func uint64Ptr(value uint64) *uint64 { return &value }

func float64Ptr(value float64) *float64 { return &value }

func collectRouteCounts(counts *Counts, metrics []*dto.Metric) error {
	seen := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		value := metric.GetCounter().GetValue()
		outcome, err := exactOutcome(metric.GetLabel())
		if err != nil {
			return err
		}
		if _, exists := seen[outcome]; exists {
			return fmt.Errorf("Prometheus route outcome is duplicated")
		}
		seen[outcome] = struct{}{}
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

func exactOutcome(labels []*dto.LabelPair) (string, error) {
	if len(labels) != 1 || labels[0].GetName() != "outcome" || labels[0].GetValue() == "" {
		return "", fmt.Errorf("Prometheus hydration metric labels are invalid")
	}
	return labels[0].GetValue(), nil
}

func validateHistogram(histogram *dto.Histogram) error {
	if histogram.GetSampleCount() == 0 || len(histogram.GetBucket()) == 0 {
		return fmt.Errorf("Prometheus hit latency histogram is empty")
	}
	var previous float64
	for index, bucket := range histogram.GetBucket() {
		upper := bucket.GetUpperBound()
		if math.IsNaN(upper) || (index > 0 && upper <= previous) || (index > 0 && bucket.GetCumulativeCount() < histogram.GetBucket()[index-1].GetCumulativeCount()) {
			return fmt.Errorf("Prometheus hit latency histogram is not monotonic")
		}
		previous = upper
	}
	return nil
}
