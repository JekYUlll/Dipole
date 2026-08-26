package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	cassandraData "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

type primaryStore struct {
	application.MessageStore
	page        []*model.Message
	err         error
	seqCalls    int
	beforeCalls int
	lastAfter   uint64
	lastBefore  uint64
}

func (s *primaryStore) ListByConversationSeqAfter(_ string, afterSeq uint64, _ int) ([]*model.Message, error) {
	s.seqCalls++
	s.lastAfter = afterSeq
	return s.page, s.err
}

func (s *primaryStore) ListByConversationSeqBefore(_ string, beforeSeq uint64, _ int) ([]*model.Message, error) {
	s.beforeCalls++
	s.lastBefore = beforeSeq
	return s.page, s.err
}

type highWaterReader struct {
	sequence uint64
	err      error
	calls    int
}

func (r *highWaterReader) LatestConversationSequence(string) (uint64, error) {
	r.calls++
	return r.sequence, r.err
}

type timelineReader struct {
	records []cassandraData.TimelineRecord
	err     error
	calls   int
	first   uint64
	last    uint64
}

func (r *timelineReader) ListRange(_ context.Context, _ string, first, last uint64) ([]cassandraData.TimelineRecord, error) {
	r.calls++
	r.first, r.last = first, last
	return r.records, r.err
}

func TestCassandraReadRouterKeepsZeroPercentOnMySQL(t *testing.T) {
	primary := &primaryStore{page: []*model.Message{{Seq: 2, Content: "mysql"}}}
	highWater := &highWaterReader{sequence: 2}
	timeline := &timelineReader{}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStore(primary, highWater, timeline, 0, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqAfter("group:G1", 1, 20)
	if err != nil || len(page) != 1 || page[0].Content != "mysql" {
		t.Fatalf("unexpected MySQL page=%+v err=%v", page, err)
	}
	if primary.seqCalls != 1 || highWater.calls != 0 || timeline.calls != 0 {
		t.Fatalf("unexpected calls primary=%d head=%d timeline=%d", primary.seqCalls, highWater.calls, timeline.calls)
	}
	if observation := <-observations; observation.Route != "mysql" || observation.ResultCount != 1 {
		t.Fatalf("unexpected MySQL observation: %+v", observation)
	}
}

func TestCassandraReadRouterServesCompleteContinuousPage(t *testing.T) {
	primary := &primaryStore{page: []*model.Message{{Seq: 2, Content: "mysql"}}}
	highWater := &highWaterReader{sequence: 4}
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{
		timelineRecord(2, "two"), timelineRecord(3, "three"), timelineRecord(4, "four"),
	}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStore(primary, highWater, timeline, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqAfter("group:G1", 1, 3)
	if err != nil || len(page) != 3 || page[0].Content != "two" || page[2].Seq != 4 {
		t.Fatalf("unexpected Cassandra page=%+v err=%v", page, err)
	}
	if primary.seqCalls != 0 || highWater.calls != 1 || timeline.calls != 1 || timeline.first != 2 || timeline.last != 4 {
		t.Fatalf("unexpected calls primary=%d head=%d timeline=%d range=%d..%d", primary.seqCalls, highWater.calls, timeline.calls, timeline.first, timeline.last)
	}
	observation := <-observations
	if observation.Route != "cassandra" || observation.FallbackReason != "" {
		t.Fatalf("unexpected observation: %+v", observation)
	}
}

func TestCassandraReadRouterServesLatestAndEarlierBeforeSequencePages(t *testing.T) {
	for _, test := range []struct {
		name        string
		beforeSeq   uint64
		first, last uint64
	}{
		{name: "latest", beforeSeq: 0, first: 3, last: 5},
		{name: "earlier", beforeSeq: 5, first: 2, last: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			primary := &primaryStore{page: []*model.Message{{Seq: test.last, Content: "mysql"}}}
			timeline := &timelineReader{records: []cassandraData.TimelineRecord{
				timelineRecord(test.first, "first"), timelineRecord(test.first+1, "middle"), timelineRecord(test.last, "last"),
			}}
			observations := make(chan ReadObservation, 1)
			store := NewMessageStore(primary, &highWaterReader{sequence: 5}, timeline, 100, func(observation ReadObservation) { observations <- observation })

			page, err := store.ListByConversationSeqBefore("group:G1", test.beforeSeq, 3)
			if err != nil || len(page) != 3 || page[0].Seq != test.first || page[2].Seq != test.last {
				t.Fatalf("unexpected Cassandra before page=%+v err=%v", page, err)
			}
			if primary.beforeCalls != 0 || timeline.first != test.first || timeline.last != test.last {
				t.Fatalf("unexpected route calls primary=%d range=%d..%d", primary.beforeCalls, timeline.first, timeline.last)
			}
			observation := <-observations
			if observation.Route != "cassandra" || observation.Operation != "before_seq" || observation.BeforeSeq != test.beforeSeq {
				t.Fatalf("unexpected observation: %+v", observation)
			}
		})
	}
}

func TestCassandraReadRouterBeforeSequenceFallsBackAsOnePage(t *testing.T) {
	primary := &primaryStore{page: []*model.Message{{ID: 10, Seq: 3, Content: "mysql-three"}, {ID: 11, Seq: 4, Content: "mysql-four"}}}
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(2, "two"), timelineRecord(4, "four")}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStore(primary, &highWaterReader{sequence: 5}, timeline, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqBefore("group:G1", 5, 3)
	if err != nil || len(page) != 2 || page[0].ID == 0 || primary.beforeCalls != 1 {
		t.Fatalf("unexpected fallback page=%+v err=%v calls=%d", page, err, primary.beforeCalls)
	}
	observation := <-observations
	if observation.Route != "mysql_fallback" || observation.FallbackReason != "incomplete_page" || observation.Operation != "before_seq" {
		t.Fatalf("unexpected observation: %+v", observation)
	}
}

func TestCassandraReadRouterBeforeSequenceEmptyBoundariesAvoidTimelineRead(t *testing.T) {
	for _, test := range []struct {
		name      string
		beforeSeq uint64
		highWater uint64
	}{
		{name: "before first sequence", beforeSeq: 1, highWater: 5},
		{name: "empty conversation", beforeSeq: 0, highWater: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			timeline := &timelineReader{}
			observations := make(chan ReadObservation, 1)
			store := NewMessageStore(&primaryStore{}, &highWaterReader{sequence: test.highWater}, timeline, 100, func(observation ReadObservation) { observations <- observation })

			page, err := store.ListByConversationSeqBefore("group:G1", test.beforeSeq, 20)
			if err != nil || len(page) != 0 || timeline.calls != 0 {
				t.Fatalf("unexpected empty page=%+v err=%v timeline_calls=%d", page, err, timeline.calls)
			}
			if observation := <-observations; observation.Route != "cassandra" || observation.ResultCount != 0 {
				t.Fatalf("unexpected observation: %+v", observation)
			}
		})
	}
}

func TestCassandraReadRouterVerificationMatchesMySQLPayload(t *testing.T) {
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(2, "same")}}
	mysqlMessage := recordsToMessages(timeline.records)[0]
	mysqlMessage.ID = 21
	primary := &primaryStore{page: []*model.Message{mysqlMessage}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStoreWithVerification(primary, &highWaterReader{sequence: 2}, timeline, 100, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqAfter("group:G1", 1, 20)
	if err != nil || len(page) != 1 || page[0].ID != 0 || primary.seqCalls != 1 {
		t.Fatalf("unexpected verified page=%+v err=%v mysql_calls=%d", page, err, primary.seqCalls)
	}
	observation := <-observations
	if observation.Route != "cassandra" || observation.VerificationOutcome != "match" {
		t.Fatalf("unexpected verification observation: %+v", observation)
	}
}

func TestCassandraReadRouterVerificationFallsBackOnPayloadMismatch(t *testing.T) {
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(2, "corrupt")}}
	mysqlMessage := recordsToMessages(timeline.records)[0]
	mysqlMessage.ID = 21
	mysqlMessage.Content = "mysql"
	primary := &primaryStore{page: []*model.Message{mysqlMessage}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStoreWithVerification(primary, &highWaterReader{sequence: 2}, timeline, 100, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqAfter("group:G1", 1, 20)
	if err != nil || len(page) != 1 || page[0].ID == 0 || page[0].Content != "mysql" {
		t.Fatalf("unexpected mismatch fallback page=%+v err=%v", page, err)
	}
	observation := <-observations
	if observation.Route != "mysql_fallback" || observation.FallbackReason != "payload_mismatch" || observation.VerificationOutcome != "mismatch" {
		t.Fatalf("unexpected mismatch observation: %+v", observation)
	}
}

func TestCassandraReadRouterVerificationErrorKeepsAvailableCassandraPage(t *testing.T) {
	primary := &primaryStore{err: errors.New("mysql unavailable")}
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(2, "available")}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStoreWithVerification(primary, &highWaterReader{sequence: 2}, timeline, 100, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqAfter("group:G1", 1, 20)
	if err != nil || len(page) != 1 || page[0].Content != "available" {
		t.Fatalf("verification dependency changed available response: page=%+v err=%v", page, err)
	}
	observation := <-observations
	if observation.Route != "cassandra" || observation.VerificationOutcome != "mysql_error" {
		t.Fatalf("unexpected verification error observation: %+v", observation)
	}
}

func TestCassandraReadRouterVerifiesBeforeSequenceWithSameCursor(t *testing.T) {
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(4, "cassandra")}}
	mysqlMessage := recordsToMessages(timeline.records)[0]
	mysqlMessage.ID = 41
	mysqlMessage.Content = "mysql"
	primary := &primaryStore{page: []*model.Message{mysqlMessage}}
	observations := make(chan ReadObservation, 1)
	store := NewMessageStoreWithVerification(primary, &highWaterReader{sequence: 5}, timeline, 100, 100, func(observation ReadObservation) { observations <- observation })

	page, err := store.ListByConversationSeqBefore("group:G1", 5, 1)
	if err != nil || len(page) != 1 || page[0].Content != "mysql" || primary.lastBefore != 5 {
		t.Fatalf("unexpected before verification page=%+v err=%v cursor=%d", page, err, primary.lastBefore)
	}
	if observation := <-observations; observation.FallbackReason != "payload_mismatch" || observation.Operation != "before_seq" {
		t.Fatalf("unexpected before observation: %+v", observation)
	}
}

func TestCassandraReadRouterFallsBackForEveryUnsafeOutcome(t *testing.T) {
	tests := []struct {
		name    string
		headErr error
		readErr error
		records []cassandraData.TimelineRecord
		reason  string
	}{
		{name: "metadata", headErr: errors.New("head unavailable"), reason: "high_watermark_error"},
		{name: "cassandra", readErr: errors.New("Cassandra unavailable"), reason: "cassandra_error"},
		{name: "missing", records: []cassandraData.TimelineRecord{timelineRecord(2, "two"), timelineRecord(4, "four")}, reason: "incomplete_page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &primaryStore{page: []*model.Message{{Seq: 2, Content: "mysql"}}}
			highWater := &highWaterReader{sequence: 4, err: tt.headErr}
			timeline := &timelineReader{records: tt.records, err: tt.readErr}
			observations := make(chan ReadObservation, 1)
			store := NewMessageStore(primary, highWater, timeline, 100, func(observation ReadObservation) { observations <- observation })

			page, err := store.ListByConversationSeqAfter("group:G1", 1, 3)
			if err != nil || len(page) != 1 || page[0].Content != "mysql" || primary.seqCalls != 1 {
				t.Fatalf("fallback page=%+v err=%v primary_calls=%d", page, err, primary.seqCalls)
			}
			observation := <-observations
			if observation.Route != "mysql_fallback" || observation.FallbackReason != tt.reason {
				t.Fatalf("unexpected observation: %+v", observation)
			}
		})
	}
}

func TestConversationCohortIsDeterministic(t *testing.T) {
	first := conversationInCohort("group:G-repeat", 37)
	for range 100 {
		if conversationInCohort("group:G-repeat", 37) != first {
			t.Fatal("conversation cohort changed across calls")
		}
	}
	if conversationInCohort("group:G1", 0) || !conversationInCohort("group:G1", 100) {
		t.Fatal("cohort percentage boundaries are incorrect")
	}
}

func TestCassandraReadRouterExportsRouteMetrics(t *testing.T) {
	primary := &primaryStore{page: []*model.Message{{Seq: 2, Content: "mysql"}}}
	store := NewMessageStore(
		primary, &highWaterReader{sequence: 2},
		&timelineReader{err: errors.New("unavailable")}, 100, func(ReadObservation) {},
	)
	_, _ = store.ListByConversationSeqAfter("group:G1", 1, 20)
	registry := prometheus.NewRegistry()
	registry.MustRegister(store)
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather route metrics: %v", err)
	}
	if len(families) != 2 {
		t.Fatalf("expected two metric families, got %d", len(families))
	}
	var fallback float64
	for _, family := range families {
		if family.GetName() != "dipole_message_read_route_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricHasLabels(metric.GetLabel(), map[string]string{"route": "mysql_fallback", "fallback_reason": "cassandra_error"}) {
				fallback = metric.GetCounter().GetValue()
			}
		}
	}
	if fallback != 1 {
		t.Fatalf("expected one Cassandra fallback, got %v", fallback)
	}
}

func TestCassandraReadRouterExportsVerificationMetrics(t *testing.T) {
	timeline := &timelineReader{records: []cassandraData.TimelineRecord{timelineRecord(2, "same")}}
	mysqlMessage := recordsToMessages(timeline.records)[0]
	mysqlMessage.ID = 21
	store := NewMessageStoreWithVerification(
		&primaryStore{page: []*model.Message{mysqlMessage}}, &highWaterReader{sequence: 2},
		timeline, 100, 100, func(ReadObservation) {},
	)
	_, _ = store.ListByConversationSeqAfter("group:G1", 1, 20)
	registry := prometheus.NewRegistry()
	registry.MustRegister(store)
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather verification metrics: %v", err)
	}
	var matched float64
	for _, family := range families {
		if family.GetName() != "dipole_message_read_verification_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricHasLabels(metric.GetLabel(), map[string]string{"operation": "after_seq", "outcome": "match"}) {
				matched = metric.GetCounter().GetValue()
			}
		}
	}
	if matched != 1 {
		t.Fatalf("expected one verified matching page, got %v", matched)
	}
}

func metricHasLabels(labels []*io_prometheus_client.LabelPair, expected map[string]string) bool {
	for _, label := range labels {
		if value, ok := expected[label.GetName()]; !ok || value != label.GetValue() {
			return false
		}
	}
	return len(labels) == len(expected)
}

func timelineRecord(seq uint64, content string) cassandraData.TimelineRecord {
	return cassandraData.TimelineRecord{Projection: cassandraData.TimelineProjection{
		ConversationKey: "group:G1", MessageSeq: seq, MessageUUID: content,
		TargetType: model.MessageTargetGroup, TargetUUID: "G1", Content: content,
		SentAt: time.Date(2026, 8, 27, 12, int(seq), 0, 0, time.UTC),
	}}
}
