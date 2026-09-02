package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"go.uber.org/zap"
)

const (
	outboxBatchSize    = 100
	outboxPollInterval = 100 * time.Millisecond
	outboxClaimLease   = 5 * time.Second
	outboxRetryBackoff = 500 * time.Millisecond
)

// outboxRelay drains pending outbox rows and publishes them to Kafka.
// The worker owns retry timing in the database so a process restart can resume
// from durable state without replay gaps.
type Relay struct {
	repo   application.OutboxRelayStore
	stopCh chan struct{}
	stop   sync.Once
	work   sync.WaitGroup
}

func NewRelay(repo application.OutboxRelayStore) *Relay {
	if repo == nil || platformKafka.Client == nil {
		return nil
	}

	return &Relay{
		repo:   repo,
		stopCh: make(chan struct{}),
	}
}

func (r *Relay) Start() {
	if r == nil {
		return
	}

	r.work.Add(1)
	go func() {
		defer r.work.Done()
		r.loop()
	}()
}

func (r *Relay) Stop() {
	if r == nil {
		return
	}

	r.stop.Do(func() { close(r.stopCh) })
	r.work.Wait()
}

func (r *Relay) loop() {
	ticker := time.NewTicker(outboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.flushBatch()
		case <-r.stopCh:
			return
		}
	}
}

func (r *Relay) flushBatch() {
	events, err := r.repo.ClaimPendingBatch(outboxBatchSize, time.Now().UTC(), outboxClaimLease)
	if err != nil {
		logger.Warn("claim outbox batch failed", zap.Error(err))
		return
	}

	for _, event := range events {
		if event == nil {
			continue
		}

		headers, err := r.repo.DecodeHeaders(event)
		if err != nil {
			logger.Warn("decode outbox headers failed",
				zap.Uint("outbox_id", event.ID),
				zap.Error(err),
			)
			_ = r.repo.MarkRetry(event.ID, event.RetryCount+1, time.Now().UTC().Add(outboxRetryBackoff), err)
			continue
		}

		err = platformKafka.Client.Publish(context.Background(), event.Topic, platformKafka.Message{
			Key:     []byte(event.MessageKey),
			Value:   event.Value,
			Headers: headers,
		})
		if err != nil {
			logger.Warn("publish outbox event failed",
				zap.Uint("outbox_id", event.ID),
				zap.String("topic", event.Topic),
				zap.Error(err),
			)
			_ = r.repo.MarkRetry(event.ID, event.RetryCount+1, time.Now().UTC().Add(outboxRetryBackoff), err)
			continue
		}

		if err := r.repo.MarkPublished(event.ID, time.Now().UTC()); err != nil {
			logger.Warn("mark outbox published failed",
				zap.Uint("outbox_id", event.ID),
				zap.Error(err),
			)
		}
	}
}
