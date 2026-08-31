package cassandrareadrollout

import (
	"bytes"
	"fmt"
	"io"
	"math"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	readRouteMetricName        = "dipole_message_read_route_total"
	readDurationMetricName     = "dipole_message_read_route_duration_seconds"
	readVerificationMetricName = "dipole_message_read_verification_total"
)

// PrometheusSnapshotMetadata binds a metrics window to the operator's deployed revision.
type PrometheusSnapshotMetadata struct {
	Service                  string
	DeploymentRevision       string
	ConfiguredReadPercentage int
	WindowStart              string
	WindowEnd                string
}

// EvidenceFromPrometheusWindow converts cumulative Message Service metrics into bounded evidence.
func EvidenceFromPrometheusWindow(startData, endData []byte, metadata PrometheusSnapshotMetadata) (Evidence, error) {
	start, err := parsePrometheusSnapshot(startData, false)
	if err != nil {
		return Evidence{}, fmt.Errorf("parse start snapshot: %w", err)
	}
	end, err := parsePrometheusSnapshot(endData, true)
	if err != nil {
		return Evidence{}, fmt.Errorf("parse end snapshot: %w", err)
	}
	routes, err := subtractRouteCounts(end.routes, start.routes)
	if err != nil {
		return Evidence{}, err
	}
	verification, err := subtractVerificationCounts(end.verification, start.verification)
	if err != nil {
		return Evidence{}, err
	}
	cassandraLatency, err := subtractHistogram(end.histograms["cassandra"], start.histograms["cassandra"])
	if err != nil {
		return Evidence{}, fmt.Errorf("subtract Cassandra latency histogram: %w", err)
	}
	mysqlLatency, err := mergeWindowHistograms(
		end.histograms["mysql"], start.histograms["mysql"],
		end.histograms["mysql_fallback"], start.histograms["mysql_fallback"],
	)
	if err != nil {
		return Evidence{}, fmt.Errorf("subtract MySQL latency histograms: %w", err)
	}
	cassandraP95, err := histogramP95Micros(cassandraLatency, routes.Cassandra)
	if err != nil {
		return Evidence{}, err
	}
	mysqlP95, err := histogramP95Micros(mysqlLatency, routes.MySQL)
	if err != nil {
		return Evidence{}, err
	}
	evidence := Evidence{
		SchemaVersion: "dipole.cassandra-read-rollout-evidence.v1", Service: metadata.Service,
		DeploymentRevision: metadata.DeploymentRevision, WindowStart: metadata.WindowStart, WindowEnd: metadata.WindowEnd,
		ConfiguredReadPercentage: metadata.ConfiguredReadPercentage,
		Requests: RouteCounts{Total: routes.Total, Cassandra: routes.Cassandra, MySQL: routes.MySQL, Fallback: routes.Fallback,
			VerificationSamples: verification.samples, VerificationMismatch: verification.mismatch, VerificationErrors: verification.errors},
		Latency: RouteLatency{CassandraP95Micros: cassandraP95, MySQLP95Micros: mysqlP95},
	}
	if err := validateEvidence(evidence); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

type readRouteCounts struct{ Total, Cassandra, MySQL, Fallback uint64 }
type verificationCounts struct{ samples, mismatch, errors uint64 }
type prometheusSnapshot struct {
	routes       readRouteCounts
	verification verificationCounts
	histograms   map[string]*dto.Histogram
}

func parsePrometheusSnapshot(data []byte, requireRequests bool) (prometheusSnapshot, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return prometheusSnapshot{}, fmt.Errorf("Prometheus snapshot is empty")
	}
	snapshot := prometheusSnapshot{histograms: make(map[string]*dto.Histogram)}
	seenFamilies := make(map[string]bool, 3)
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
		case readRouteMetricName:
			if seenFamilies[readRouteMetricName] || family.GetType() != dto.MetricType_COUNTER {
				return prometheusSnapshot{}, fmt.Errorf("Prometheus read route family is duplicated or has the wrong type")
			}
			seenFamilies[readRouteMetricName] = true
			if err := collectReadRoutes(&snapshot.routes, family.GetMetric()); err != nil {
				return prometheusSnapshot{}, err
			}
		case readVerificationMetricName:
			if seenFamilies[readVerificationMetricName] || family.GetType() != dto.MetricType_COUNTER {
				return prometheusSnapshot{}, fmt.Errorf("Prometheus verification family is duplicated or has the wrong type")
			}
			seenFamilies[readVerificationMetricName] = true
			if err := collectVerificationCounts(&snapshot.verification, family.GetMetric()); err != nil {
				return prometheusSnapshot{}, err
			}
		case readDurationMetricName:
			if seenFamilies[readDurationMetricName] || family.GetType() != dto.MetricType_HISTOGRAM {
				return prometheusSnapshot{}, fmt.Errorf("Prometheus read duration family is duplicated or has the wrong type")
			}
			seenFamilies[readDurationMetricName] = true
			if err := collectRouteHistograms(snapshot.histograms, family.GetMetric()); err != nil {
				return prometheusSnapshot{}, err
			}
		}
	}
	if !seenFamilies[readRouteMetricName] || (requireRequests && snapshot.routes.Total == 0) {
		return prometheusSnapshot{}, fmt.Errorf("Prometheus snapshot has no message read routes")
	}
	return snapshot, nil
}

func collectReadRoutes(counts *readRouteCounts, metrics []*dto.Metric) error {
	seen := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		route, fallbackReason, err := readRouteLabels(metric.GetLabel())
		if err != nil || metric.Counter == nil || !wholeCounter(metric.GetCounter().GetValue()) {
			return fmt.Errorf("Prometheus read route metric is invalid")
		}
		key := route + "\x00" + fallbackReason
		if _, exists := seen[key]; exists {
			return fmt.Errorf("Prometheus read route labels are duplicated")
		}
		seen[key] = struct{}{}
		count := uint64(metric.GetCounter().GetValue())
		switch route {
		case "cassandra":
			if fallbackReason != "" {
				return fmt.Errorf("Cassandra route cannot have a fallback reason")
			}
			counts.Cassandra += count
		case "mysql":
			if fallbackReason != "" {
				return fmt.Errorf("MySQL route cannot have a fallback reason")
			}
			counts.MySQL += count
		case "mysql_fallback":
			if fallbackReason == "" {
				return fmt.Errorf("MySQL fallback route requires a reason")
			}
			counts.MySQL += count
			counts.Fallback += count
		default:
			return fmt.Errorf("Prometheus read route is unsupported")
		}
		counts.Total += count
	}
	return nil
}

func collectVerificationCounts(counts *verificationCounts, metrics []*dto.Metric) error {
	seen := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		operation, outcome, err := verificationLabels(metric.GetLabel())
		if err != nil || metric.Counter == nil || !wholeCounter(metric.GetCounter().GetValue()) {
			return fmt.Errorf("Prometheus verification metric is invalid")
		}
		key := operation + "\x00" + outcome
		if _, exists := seen[key]; exists {
			return fmt.Errorf("Prometheus verification labels are duplicated")
		}
		seen[key] = struct{}{}
		count := uint64(metric.GetCounter().GetValue())
		switch outcome {
		case "match":
			counts.samples += count
		case "mismatch":
			counts.samples += count
			counts.mismatch += count
		case "mysql_error":
			counts.samples += count
			counts.errors += count
		default:
			return fmt.Errorf("Prometheus verification outcome is unsupported")
		}
	}
	return nil
}

func collectRouteHistograms(histograms map[string]*dto.Histogram, metrics []*dto.Metric) error {
	for _, metric := range metrics {
		route, err := histogramRouteLabel(metric.GetLabel())
		if err != nil || metric.Histogram == nil {
			return fmt.Errorf("Prometheus read duration metric is invalid")
		}
		if _, exists := histograms[route]; exists {
			return fmt.Errorf("Prometheus read duration route is duplicated")
		}
		if route != "cassandra" && route != "mysql" && route != "mysql_fallback" {
			return fmt.Errorf("Prometheus duration route is unsupported")
		}
		if err := validateHistogram(metric.GetHistogram()); err != nil {
			return err
		}
		histograms[route] = metric.GetHistogram()
	}
	return nil
}

func subtractRouteCounts(end, start readRouteCounts) (readRouteCounts, error) {
	if end.Total < start.Total || end.Cassandra < start.Cassandra || end.MySQL < start.MySQL || end.Fallback < start.Fallback {
		return readRouteCounts{}, fmt.Errorf("Prometheus read route counters moved backwards")
	}
	return readRouteCounts{Total: end.Total - start.Total, Cassandra: end.Cassandra - start.Cassandra, MySQL: end.MySQL - start.MySQL, Fallback: end.Fallback - start.Fallback}, nil
}

func subtractVerificationCounts(end, start verificationCounts) (verificationCounts, error) {
	if end.samples < start.samples || end.mismatch < start.mismatch || end.errors < start.errors {
		return verificationCounts{}, fmt.Errorf("Prometheus verification counters moved backwards")
	}
	return verificationCounts{samples: end.samples - start.samples, mismatch: end.mismatch - start.mismatch, errors: end.errors - start.errors}, nil
}

func mergeWindowHistograms(endFirst, startFirst, endSecond, startSecond *dto.Histogram) (*dto.Histogram, error) {
	first, err := subtractHistogram(endFirst, startFirst)
	if err != nil {
		return nil, err
	}
	second, err := subtractHistogram(endSecond, startSecond)
	if err != nil {
		return nil, err
	}
	return mergeHistograms(first, second)
}

func subtractHistogram(end, start *dto.Histogram) (*dto.Histogram, error) {
	if end == nil && start == nil {
		return nil, nil
	}
	if start == nil {
		if end == nil {
			return nil, nil
		}
		return cloneHistogram(end), nil
	}
	if end == nil || end.GetSampleCount() < start.GetSampleCount() || len(end.GetBucket()) != len(start.GetBucket()) {
		return nil, fmt.Errorf("histogram is missing, reset, or changed")
	}
	result := &dto.Histogram{SampleCount: uint64Ptr(end.GetSampleCount() - start.GetSampleCount()), SampleSum: float64Ptr(end.GetSampleSum() - start.GetSampleSum())}
	for index, bucket := range end.GetBucket() {
		previous := start.GetBucket()[index]
		if bucket.GetUpperBound() != previous.GetUpperBound() || bucket.GetCumulativeCount() < previous.GetCumulativeCount() {
			return nil, fmt.Errorf("histogram buckets moved backwards or changed")
		}
		result.Bucket = append(result.Bucket, &dto.Bucket{UpperBound: float64Ptr(bucket.GetUpperBound()), CumulativeCount: uint64Ptr(bucket.GetCumulativeCount() - previous.GetCumulativeCount())})
	}
	return result, nil
}

func cloneHistogram(histogram *dto.Histogram) *dto.Histogram {
	result := &dto.Histogram{SampleCount: uint64Ptr(histogram.GetSampleCount()), SampleSum: float64Ptr(histogram.GetSampleSum())}
	for _, bucket := range histogram.GetBucket() {
		result.Bucket = append(result.Bucket, &dto.Bucket{UpperBound: float64Ptr(bucket.GetUpperBound()), CumulativeCount: uint64Ptr(bucket.GetCumulativeCount())})
	}
	return result
}

func mergeHistograms(first, second *dto.Histogram) (*dto.Histogram, error) {
	if first == nil {
		return second, nil
	}
	if second == nil {
		return first, nil
	}
	if len(first.GetBucket()) != len(second.GetBucket()) {
		return nil, fmt.Errorf("histogram buckets differ across MySQL routes")
	}
	result := &dto.Histogram{SampleCount: uint64Ptr(first.GetSampleCount() + second.GetSampleCount()), SampleSum: float64Ptr(first.GetSampleSum() + second.GetSampleSum())}
	for index, bucket := range first.GetBucket() {
		other := second.GetBucket()[index]
		if bucket.GetUpperBound() != other.GetUpperBound() {
			return nil, fmt.Errorf("histogram bucket bounds differ across MySQL routes")
		}
		result.Bucket = append(result.Bucket, &dto.Bucket{UpperBound: float64Ptr(bucket.GetUpperBound()), CumulativeCount: uint64Ptr(bucket.GetCumulativeCount() + other.GetCumulativeCount())})
	}
	return result, nil
}

func histogramP95Micros(histogram *dto.Histogram, requests uint64) (uint64, error) {
	if requests == 0 {
		return 0, nil
	}
	if histogram == nil || histogram.GetSampleCount() != requests {
		return 0, fmt.Errorf("Prometheus latency histogram does not cover all routed reads")
	}
	rank := (requests*95 + 99) / 100
	for _, bucket := range histogram.GetBucket() {
		if bucket.GetCumulativeCount() < rank {
			continue
		}
		upper := bucket.GetUpperBound()
		if math.IsInf(upper, 0) || upper < 0 || upper > float64(math.MaxUint64)/1e6 {
			return 0, fmt.Errorf("Prometheus latency p95 bucket is invalid")
		}
		return uint64(math.Ceil(upper * 1e6)), nil
	}
	return 0, fmt.Errorf("Prometheus latency histogram has no finite p95 bucket")
}

func readRouteLabels(labels []*dto.LabelPair) (string, string, error) {
	if len(labels) != 2 {
		return "", "", fmt.Errorf("invalid route labels")
	}
	values := labelValues(labels)
	if len(values) != 2 || values["route"] == "" {
		return "", "", fmt.Errorf("invalid route labels")
	}
	return values["route"], values["fallback_reason"], nil
}

func verificationLabels(labels []*dto.LabelPair) (string, string, error) {
	if len(labels) != 2 {
		return "", "", fmt.Errorf("invalid verification labels")
	}
	values := labelValues(labels)
	if len(values) != 2 || (values["operation"] != "after_seq" && values["operation"] != "before_seq") || values["outcome"] == "" {
		return "", "", fmt.Errorf("invalid verification labels")
	}
	return values["operation"], values["outcome"], nil
}

func histogramRouteLabel(labels []*dto.LabelPair) (string, error) {
	if len(labels) != 1 || labels[0].GetName() != "route" || labels[0].GetValue() == "" {
		return "", fmt.Errorf("invalid duration labels")
	}
	return labels[0].GetValue(), nil
}

func labelValues(labels []*dto.LabelPair) map[string]string {
	values := make(map[string]string, len(labels))
	for _, label := range labels {
		values[label.GetName()] = label.GetValue()
	}
	return values
}

func wholeCounter(value float64) bool {
	return value >= 0 && math.Trunc(value) == value && value <= float64(^uint64(0))
}
func uint64Ptr(value uint64) *uint64    { return &value }
func float64Ptr(value float64) *float64 { return &value }

func validateHistogram(histogram *dto.Histogram) error {
	if histogram.GetSampleCount() == 0 || len(histogram.GetBucket()) == 0 {
		return fmt.Errorf("Prometheus latency histogram is empty")
	}
	var previous float64
	for index, bucket := range histogram.GetBucket() {
		upper := bucket.GetUpperBound()
		if math.IsNaN(upper) || (index > 0 && upper <= previous) || (index > 0 && bucket.GetCumulativeCount() < histogram.GetBucket()[index-1].GetCumulativeCount()) {
			return fmt.Errorf("Prometheus latency histogram is not monotonic")
		}
		previous = upper
	}
	return nil
}
