// Package agenttimeline contains explicit repair workers for Agent Task Timeline projections.
package agenttimeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type Report struct {
	Claimed  int
	Repaired int
	Retried  int
}

type Observer interface {
	Observe(outcome string, duration time.Duration)
}

type Repairer struct {
	repairs      application.AgentTaskTimelineRepairStoreV1
	timeline     application.AgentTaskTimelineStoreV1
	batchSize    int
	lease        time.Duration
	retryBackoff time.Duration
	interval     time.Duration
	observer     Observer
}

func (r *Repairer) WithObserver(observer Observer) *Repairer {
	if r != nil {
		r.observer = observer
	}
	return r
}

func NewRepairer(repairs application.AgentTaskTimelineRepairStoreV1, timeline application.AgentTaskTimelineStoreV1, batchSize int, lease, retryBackoff, interval time.Duration) (*Repairer, error) {
	if repairs == nil || timeline == nil {
		return nil, errors.New("Agent Task Timeline repair stores are required")
	}
	if batchSize < 1 || batchSize > 500 {
		return nil, errors.New("Agent Task Timeline repair batch size is invalid")
	}
	if lease <= 0 || retryBackoff <= 0 || interval <= 0 {
		return nil, errors.New("Agent Task Timeline repair timings are invalid")
	}
	return &Repairer{repairs: repairs, timeline: timeline, batchSize: batchSize, lease: lease, retryBackoff: retryBackoff, interval: interval}, nil
}

func (r *Repairer) RunOnce(ctx context.Context, now time.Time) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	started := time.Now()
	now = now.UTC()
	items, err := r.repairs.ClaimAgentTaskTimelineRepairs(r.batchSize, now, r.lease)
	if err != nil {
		r.observe("claim_error", time.Since(started))
		return Report{}, fmt.Errorf("claim Agent Task Timeline repairs: %w", err)
	}
	report := Report{Claimed: len(items)}
	var firstErr error
	for _, item := range items {
		itemStarted := time.Now()
		event := application.AgentTaskTimelineEventV1{
			EventUUID: item.EventUUID, TaskUUID: item.TaskUUID, RunUUID: item.RunUUID,
			Kind: item.Kind, Status: item.Status, CapabilityID: item.CapabilityID,
			ApprovalUUID: item.ApprovalUUID, ArtifactUUID: item.ArtifactUUID, OccurredAt: item.OccurredAt,
		}
		if err := event.Validate(); err != nil {
			r.observe("invalid", time.Since(itemStarted))
			firstErr = joinFirst(firstErr, fmt.Errorf("validate repair %s: %w", item.EventUUID, err))
			continue
		}
		if _, err := r.timeline.AppendAgentTaskTimelineEvent(ctx, event); err != nil {
			report.Retried++
			next := now.Add(r.retryBackoff)
			if retryErr := r.repairs.MarkAgentTaskTimelineRepairRetry(item.EventUUID, item.RetryCount+1, next, err); retryErr != nil {
				r.observe("complete_error", time.Since(itemStarted))
				firstErr = joinFirst(firstErr, fmt.Errorf("retry repair %s: %w", item.EventUUID, retryErr))
			} else {
				r.observe("projection_error", time.Since(itemStarted))
			}
			continue
		}
		if err := r.repairs.MarkAgentTaskTimelineRepairCompleted(item.EventUUID); err != nil {
			r.observe("complete_error", time.Since(itemStarted))
			firstErr = joinFirst(firstErr, fmt.Errorf("complete repair %s: %w", item.EventUUID, err))
			continue
		}
		report.Repaired++
		r.observe("repaired", time.Since(itemStarted))
	}
	if report.Claimed == 0 {
		r.observe("empty", 0)
	}
	return report, firstErr
}

func (r *Repairer) observe(outcome string, duration time.Duration) {
	if r != nil && r.observer != nil {
		r.observer.Observe(outcome, duration)
	}
}

func (r *Repairer) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if _, err := r.RunOnce(ctx, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			// A failed batch remains durable and will be claimed again; keep the worker alive.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func joinFirst(first, next error) error {
	if first != nil {
		return first
	}
	return next
}
