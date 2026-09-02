package searchcutover

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Action string

const (
	ActionSwitch   Action = "switch"
	ActionRollback Action = "rollback"
)

type Source interface {
	HighWatermark(context.Context) (uint64, error)
}

type Snapshot interface {
	CompletedHighWatermark(context.Context, string) (uint64, error)
}

type Verification struct {
	Consistent  bool   `json:"consistent"`
	SourceCount uint64 `json:"source_count"`
	TargetCount uint64 `json:"target_count"`
}

type Verifier interface {
	Verify(context.Context, uint64) (Verification, error)
}

type AliasSwitcher interface {
	Switch(context.Context, string, string) error
}

type Config struct {
	Action               Action
	JobName              string
	FromIndex            string
	ToIndex              string
	MaintenanceConfirmed bool
	RollbackWindow       time.Duration
}

type Receipt struct {
	Action              Action    `json:"action"`
	JobName             string    `json:"job_name"`
	FromIndex           string    `json:"from_index"`
	ToIndex             string    `json:"to_index"`
	SourceHighWatermark uint64    `json:"source_high_watermark_id"`
	SourceCount         uint64    `json:"source_count"`
	TargetCount         uint64    `json:"target_count"`
	SwitchedAt          time.Time `json:"switched_at"`
	RollbackUntil       time.Time `json:"rollback_until"`
}

type Switcher struct {
	source   Source
	snapshot Snapshot
	verifier Verifier
	aliases  AliasSwitcher
	config   Config
	now      func() time.Time
}

func New(source Source, snapshot Snapshot, verifier Verifier, aliases AliasSwitcher, cfg Config) (*Switcher, error) {
	switch {
	case source == nil:
		return nil, errors.New("Search cutover source is required")
	case snapshot == nil:
		return nil, errors.New("Search cutover snapshot is required")
	case verifier == nil:
		return nil, errors.New("Search cutover verifier is required")
	case aliases == nil:
		return nil, errors.New("Search cutover alias switcher is required")
	case cfg.Action != ActionSwitch && cfg.Action != ActionRollback:
		return nil, errors.New("Search cutover action must be switch or rollback")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Search cutover job name is required")
	case strings.TrimSpace(cfg.FromIndex) == "" || strings.TrimSpace(cfg.ToIndex) == "" || cfg.FromIndex == cfg.ToIndex:
		return nil, errors.New("Search cutover source and target indices are invalid")
	case !cfg.MaintenanceConfirmed:
		return nil, errors.New("Search cutover requires an explicit maintenance-window confirmation")
	case cfg.RollbackWindow <= 0:
		return nil, errors.New("Search cutover rollback window must be positive")
	}
	return &Switcher{source: source, snapshot: snapshot, verifier: verifier, aliases: aliases, config: cfg, now: time.Now}, nil
}

func (s *Switcher) Run(ctx context.Context) (Receipt, error) {
	highWatermark, err := s.snapshot.CompletedHighWatermark(ctx, s.config.JobName)
	if err != nil {
		return Receipt{}, fmt.Errorf("read completed Search snapshot: %w", err)
	}
	if err := s.requireCurrentHighWatermark(ctx, highWatermark, "before reconciliation"); err != nil {
		return Receipt{}, err
	}
	verification, err := s.verifier.Verify(ctx, highWatermark)
	if err != nil {
		return Receipt{}, fmt.Errorf("verify Search cutover target: %w", err)
	}
	if !verification.Consistent {
		return Receipt{}, fmt.Errorf("Search cutover target is inconsistent: source_count=%d target_count=%d", verification.SourceCount, verification.TargetCount)
	}
	if err := s.requireCurrentHighWatermark(ctx, highWatermark, "before alias switch"); err != nil {
		return Receipt{}, err
	}
	if err := s.aliases.Switch(ctx, s.config.FromIndex, s.config.ToIndex); err != nil {
		return Receipt{}, fmt.Errorf("switch Search aliases: %w", err)
	}
	if err := s.requireCurrentHighWatermark(ctx, highWatermark, "after alias switch"); err != nil {
		compensationErr := s.aliases.Switch(ctx, s.config.ToIndex, s.config.FromIndex)
		if compensationErr != nil {
			return Receipt{}, errors.Join(err, fmt.Errorf("compensate Search alias switch: %w", compensationErr))
		}
		return Receipt{}, fmt.Errorf("%w; aliases restored to %s", err, s.config.FromIndex)
	}
	now := s.now().UTC()
	return Receipt{
		Action: s.config.Action, JobName: s.config.JobName, FromIndex: s.config.FromIndex, ToIndex: s.config.ToIndex,
		SourceHighWatermark: highWatermark, SourceCount: verification.SourceCount, TargetCount: verification.TargetCount,
		SwitchedAt: now, RollbackUntil: now.Add(s.config.RollbackWindow),
	}, nil
}

func (s *Switcher) requireCurrentHighWatermark(ctx context.Context, expected uint64, stage string) error {
	current, err := s.source.HighWatermark(ctx)
	if err != nil {
		return fmt.Errorf("read Search source high watermark %s: %w", stage, err)
	}
	if current != expected {
		return fmt.Errorf("Search snapshot became stale %s: snapshot=%d current=%d", stage, expected, current)
	}
	return nil
}
