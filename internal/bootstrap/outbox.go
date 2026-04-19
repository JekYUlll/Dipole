package bootstrap

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/repository"
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
type outboxRelay struct {
	repo   *repository.OutboxRepository
	stopCh chan struct{}
}

func newOutboxRelay(repo *repository.OutboxRepository) *outboxRelay {
	if repo == nil || platformKafka.Client == nil {
		return nil
	}

	return &outboxRelay{
		repo:   repo,
		stopCh: make(chan struct{}),
	}
}

func (r *outboxRelay) Start() {
	if r == nil {
		return
	}

	go r.loop()
}

func (r *outboxRelay) Stop() {
	if r == nil {
		return
	}

	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

func (r *outboxRelay) loop() {
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

func (r *outboxRelay) flushBatch() {
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
