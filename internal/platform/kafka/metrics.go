package kafka

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type readerMetricTotals struct {
	messages   float64
	bytes      float64
	rebalances float64
	timeouts   float64
	errors     float64
}

type ConsumerCollector struct {
	consumer *Consumer
	labels   []string

	fetched          *prometheus.Desc
	handled          *prometheus.Desc
	committed        *prometheus.Desc
	fetchErrors      *prometheus.Desc
	commitErrors     *prometheus.Desc
	retryPublished   *prometheus.Desc
	deadPublished    *prometheus.Desc
	readerMessages   *prometheus.Desc
	readerBytes      *prometheus.Desc
	readerRebalances *prometheus.Desc
	readerTimeouts   *prometheus.Desc
	readerErrors     *prometheus.Desc
	readerQueue      *prometheus.Desc

	mu           sync.Mutex
	readerTotals map[string]readerMetricTotals
}

func NewConsumerCollector(consumer *Consumer) *ConsumerCollector {
	labels := []string{"client_id", "group_id"}
	readerLabels := []string{"client_id", "group_id", "topic"}
	return &ConsumerCollector{
		consumer:         consumer,
		labels:           labels,
		fetched:          prometheus.NewDesc("dipole_kafka_consumer_fetched_total", "Kafka messages fetched by this process.", labels, nil),
		handled:          prometheus.NewDesc("dipole_kafka_consumer_handled_total", "Kafka messages handled or transferred to retry/dead-letter.", labels, nil),
		committed:        prometheus.NewDesc("dipole_kafka_consumer_committed_total", "Kafka offsets committed by this process.", labels, nil),
		fetchErrors:      prometheus.NewDesc("dipole_kafka_consumer_fetch_errors_total", "Kafka fetch errors observed by this process.", labels, nil),
		commitErrors:     prometheus.NewDesc("dipole_kafka_consumer_commit_errors_total", "Kafka offset commit errors observed by this process.", labels, nil),
		retryPublished:   prometheus.NewDesc("dipole_kafka_consumer_retry_published_total", "Messages published to retry topics.", labels, nil),
		deadPublished:    prometheus.NewDesc("dipole_kafka_consumer_dead_letter_published_total", "Messages published to dead-letter topics.", labels, nil),
		readerMessages:   prometheus.NewDesc("dipole_kafka_consumer_reader_messages_total", "Messages reported by kafka-go readers.", readerLabels, nil),
		readerBytes:      prometheus.NewDesc("dipole_kafka_consumer_reader_bytes_total", "Bytes reported by kafka-go readers.", readerLabels, nil),
		readerRebalances: prometheus.NewDesc("dipole_kafka_consumer_reader_rebalances_total", "Rebalances reported by kafka-go readers.", readerLabels, nil),
		readerTimeouts:   prometheus.NewDesc("dipole_kafka_consumer_reader_timeouts_total", "Timeouts reported by kafka-go readers.", readerLabels, nil),
		readerErrors:     prometheus.NewDesc("dipole_kafka_consumer_reader_errors_total", "Errors reported by kafka-go readers.", readerLabels, nil),
		readerQueue:      prometheus.NewDesc("dipole_kafka_consumer_reader_queue_length", "Current kafka-go reader queue length.", readerLabels, nil),
		readerTotals:     make(map[string]readerMetricTotals),
	}
}

func (c *ConsumerCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{
		c.fetched, c.handled, c.committed, c.fetchErrors, c.commitErrors,
		c.retryPublished, c.deadPublished, c.readerMessages, c.readerBytes,
		c.readerRebalances, c.readerTimeouts, c.readerErrors, c.readerQueue,
	} {
		ch <- desc
	}
}

func (c *ConsumerCollector) Collect(ch chan<- prometheus.Metric) {
	if c == nil || c.consumer == nil {
		return
	}
	stats := c.consumer.CollectStats()
	values := []string{stats.ClientID, stats.GroupID}
	ch <- prometheus.MustNewConstMetric(c.fetched, prometheus.CounterValue, float64(stats.Fetched), values...)
	ch <- prometheus.MustNewConstMetric(c.handled, prometheus.CounterValue, float64(stats.Handled), values...)
	ch <- prometheus.MustNewConstMetric(c.committed, prometheus.CounterValue, float64(stats.Committed), values...)
	ch <- prometheus.MustNewConstMetric(c.fetchErrors, prometheus.CounterValue, float64(stats.FetchErrors), values...)
	ch <- prometheus.MustNewConstMetric(c.commitErrors, prometheus.CounterValue, float64(stats.CommitErrors), values...)
	ch <- prometheus.MustNewConstMetric(c.retryPublished, prometheus.CounterValue, float64(stats.RetryPublished), values...)
	ch <- prometheus.MustNewConstMetric(c.deadPublished, prometheus.CounterValue, float64(stats.DeadPublished), values...)

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, reader := range stats.Readers {
		total := c.readerTotals[reader.Topic]
		total.messages += float64(reader.Messages)
		total.bytes += float64(reader.Bytes)
		total.rebalances += float64(reader.Rebalances)
		total.timeouts += float64(reader.Timeouts)
		total.errors += float64(reader.Errors)
		c.readerTotals[reader.Topic] = total
		readerValues := []string{stats.ClientID, stats.GroupID, reader.Topic}
		ch <- prometheus.MustNewConstMetric(c.readerMessages, prometheus.CounterValue, total.messages, readerValues...)
		ch <- prometheus.MustNewConstMetric(c.readerBytes, prometheus.CounterValue, total.bytes, readerValues...)
		ch <- prometheus.MustNewConstMetric(c.readerRebalances, prometheus.CounterValue, total.rebalances, readerValues...)
		ch <- prometheus.MustNewConstMetric(c.readerTimeouts, prometheus.CounterValue, total.timeouts, readerValues...)
		ch <- prometheus.MustNewConstMetric(c.readerErrors, prometheus.CounterValue, total.errors, readerValues...)
		ch <- prometheus.MustNewConstMetric(c.readerQueue, prometheus.GaugeValue, float64(reader.QueueLength), readerValues...)
	}
}
