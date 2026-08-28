package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCutoverControllerOwnershipUnavailable = errors.New("cutover controller ownership is unavailable")

type CutoverControllerOrchestrator interface {
	Advance(context.Context) (CutoverAttemptAdvance, error)
	RenewLease(context.Context) (CutoverAttemptAdvance, error)
}

type CutoverControllerOwnership interface {
	Acquire(context.Context, string, time.Duration) (bool, error)
	Renew(context.Context, string, time.Duration) (bool, error)
	Release(context.Context, string) error
}

type CutoverControllerAuthorityLease interface {
	CurrentLease(context.Context) (FenceTransitionReceipt, error)
}

type CutoverAttemptControllerConfig struct {
	Orchestrator   CutoverControllerOrchestrator
	Ownership      CutoverControllerOwnership
	AuthorityLease CutoverControllerAuthorityLease
	OwnerID        string
	OwnershipTTL   time.Duration
	ActionTimeout  time.Duration
	RenewBefore    time.Duration
	RetryInterval  time.Duration
	Now            func() time.Time
}

type CutoverAttemptController struct {
	config CutoverAttemptControllerConfig
}

type CutoverWorkspaceAuthorityLease struct {
	directory string
	initial   FenceTransitionReceipt
	artifacts *CutoverActionArtifactStore
}

func NewCutoverWorkspaceAuthorityLease(workspace *CutoverAttemptWorkspace) (*CutoverWorkspaceAuthorityLease, error) {
	if workspace == nil || strings.TrimSpace(workspace.Directory) == "" || workspace.Artifacts == nil {
		return nil, fmt.Errorf("cutover workspace authority lease configuration is invalid")
	}
	return &CutoverWorkspaceAuthorityLease{
		directory: workspace.Directory, initial: workspace.Inputs.InitialTransition, artifacts: workspace.Artifacts,
	}, nil
}

func (s *CutoverWorkspaceAuthorityLease) CurrentLease(ctx context.Context) (FenceTransitionReceipt, error) {
	if err := ctx.Err(); err != nil {
		return FenceTransitionReceipt{}, err
	}
	journal, err := LoadCutoverAttemptJournal(s.directory)
	if err != nil {
		return FenceTransitionReceipt{}, err
	}
	for index := len(journal.Events) - 1; index >= 0; index-- {
		event := journal.Events[index]
		if !cutoverEventChangesAuthorityLease(event.EventType) {
			continue
		}
		actionID, err := cutoverAttemptActionID(journal.Manifest.AttemptID, event.Sequence, event.EventType)
		if err != nil {
			return FenceTransitionReceipt{}, err
		}
		artifact, digest, err := s.artifacts.LoadByActionID(actionID)
		if err != nil {
			return FenceTransitionReceipt{}, err
		}
		if digest != event.ArtifactSHA256 || artifact.EventType != event.EventType || artifact.Sequence != event.Sequence {
			return FenceTransitionReceipt{}, fmt.Errorf("cutover authority lease artifact does not match journal")
		}
		receipt, err := DecodeCutoverActionArtifactPayload[FenceTransitionReceipt](artifact)
		if err != nil {
			return FenceTransitionReceipt{}, err
		}
		if receipt.TransitionID != actionID {
			return FenceTransitionReceipt{}, fmt.Errorf("cutover authority lease transition does not match action")
		}
		return receipt, nil
	}
	return s.initial, nil
}

func cutoverEventChangesAuthorityLease(eventType CutoverAttemptEventType) bool {
	switch eventType {
	case CutoverEventFreezeApplied, CutoverEventTargetActivated, CutoverEventLeaseRenewed,
		CutoverEventRollbackFreezeApplied, CutoverEventSourceReactivated:
		return true
	default:
		return false
	}
}

func NewCutoverAttemptController(config CutoverAttemptControllerConfig) (*CutoverAttemptController, error) {
	config.OwnerID = strings.TrimSpace(config.OwnerID)
	if config.Orchestrator == nil || config.Ownership == nil || config.AuthorityLease == nil || config.Now == nil ||
		!fenceTransitionIDPattern.MatchString(config.OwnerID) {
		return nil, fmt.Errorf("cutover controller configuration is invalid")
	}
	if config.ActionTimeout < time.Second || config.ActionTimeout > time.Minute ||
		config.OwnershipTTL < 2*config.ActionTimeout || config.OwnershipTTL > 10*time.Minute ||
		config.RenewBefore < time.Second || config.RenewBefore >= config.OwnershipTTL ||
		config.RetryInterval <= 0 || config.RetryInterval > time.Minute {
		return nil, fmt.Errorf("cutover controller timing configuration is invalid")
	}
	return &CutoverAttemptController{config: config}, nil
}

func (c *CutoverAttemptController) Run(ctx context.Context) (CutoverAttemptAdvance, error) {
	acquired, err := c.config.Ownership.Acquire(ctx, c.config.OwnerID, c.config.OwnershipTTL)
	if err != nil {
		return CutoverAttemptAdvance{}, fmt.Errorf("acquire cutover controller ownership: %w", err)
	}
	if !acquired {
		return CutoverAttemptAdvance{}, ErrCutoverControllerOwnershipUnavailable
	}
	defer c.releaseOwnership()

	first := true
	for {
		if err := ctx.Err(); err != nil {
			return CutoverAttemptAdvance{}, err
		}
		if !first {
			owned, renewErr := c.config.Ownership.Renew(ctx, c.config.OwnerID, c.config.OwnershipTTL)
			if renewErr != nil {
				return CutoverAttemptAdvance{}, fmt.Errorf("renew cutover controller ownership: %w", renewErr)
			}
			if !owned {
				return CutoverAttemptAdvance{}, ErrCutoverControllerOwnershipUnavailable
			}
		}
		first = false

		result, advanceErr := c.advance(ctx)
		if advanceErr == nil {
			if result.Terminal {
				return result, nil
			}
			continue
		}

		owned, renewErr := c.config.Ownership.Renew(ctx, c.config.OwnerID, c.config.OwnershipTTL)
		if renewErr != nil {
			return CutoverAttemptAdvance{}, fmt.Errorf("renew cutover controller ownership after blocked action: %w", renewErr)
		}
		if !owned {
			return CutoverAttemptAdvance{}, ErrCutoverControllerOwnershipUnavailable
		}
		if err := c.renewAuthorityIfDue(ctx); err != nil {
			advanceErr = errors.Join(advanceErr, err)
		}
		if err := waitCutoverControllerRetry(ctx, c.config.RetryInterval); err != nil {
			return CutoverAttemptAdvance{}, errors.Join(advanceErr, err)
		}
	}
}

func (c *CutoverAttemptController) advance(ctx context.Context) (CutoverAttemptAdvance, error) {
	actionCtx, cancel := context.WithTimeout(ctx, c.config.ActionTimeout)
	defer cancel()
	return c.config.Orchestrator.Advance(actionCtx)
}

func (c *CutoverAttemptController) renewAuthorityIfDue(ctx context.Context) error {
	leaseCtx, cancel := context.WithTimeout(ctx, c.config.ActionTimeout)
	lease, err := c.config.AuthorityLease.CurrentLease(leaseCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("read cutover authority lease: %w", err)
	}
	if time.UnixMilli(lease.LeaseUntilUnixMS).After(c.config.Now().UTC().Add(c.config.RenewBefore)) {
		return nil
	}
	renewCtx, renewCancel := context.WithTimeout(ctx, c.config.ActionTimeout)
	_, err = c.config.Orchestrator.RenewLease(renewCtx)
	renewCancel()
	if err != nil {
		return fmt.Errorf("renew cutover authority lease: %w", err)
	}
	return nil
}

func (c *CutoverAttemptController) releaseOwnership() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.config.Ownership.Release(ctx, c.config.OwnerID)
}

func waitCutoverControllerRetry(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type RedisCutoverControllerOwnership struct {
	client redis.Cmdable
	key    string
}

const renewCutoverControllerOwnershipScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
`

const releaseCutoverControllerOwnershipScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
redis.call('DEL', KEYS[1])
return 1
`

func NewRedisCutoverControllerOwnership(client redis.Cmdable, key string) (*RedisCutoverControllerOwnership, error) {
	key = strings.TrimSpace(key)
	if client == nil || key == "" {
		return nil, fmt.Errorf("cutover controller ownership configuration is invalid")
	}
	return &RedisCutoverControllerOwnership{client: client, key: key}, nil
}

func (o *RedisCutoverControllerOwnership) Acquire(ctx context.Context, owner string, ttl time.Duration) (bool, error) {
	if err := validateCutoverControllerOwnership(owner, ttl); err != nil {
		return false, err
	}
	acquired, err := o.client.SetNX(ctx, o.key, owner, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire Redis cutover controller ownership: %w", err)
	}
	return acquired, nil
}

func (o *RedisCutoverControllerOwnership) Renew(ctx context.Context, owner string, ttl time.Duration) (bool, error) {
	if err := validateCutoverControllerOwnership(owner, ttl); err != nil {
		return false, err
	}
	result, err := o.client.Eval(ctx, renewCutoverControllerOwnershipScript, []string{o.key}, owner, ttl.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("renew Redis cutover controller ownership: %w", err)
	}
	return result == 1, nil
}

func (o *RedisCutoverControllerOwnership) Release(ctx context.Context, owner string) error {
	if !fenceTransitionIDPattern.MatchString(strings.TrimSpace(owner)) {
		return fmt.Errorf("cutover controller owner ID is invalid")
	}
	if _, err := o.client.Eval(ctx, releaseCutoverControllerOwnershipScript, []string{o.key}, owner).Int64(); err != nil {
		return fmt.Errorf("release Redis cutover controller ownership: %w", err)
	}
	return nil
}

func validateCutoverControllerOwnership(owner string, ttl time.Duration) error {
	if !fenceTransitionIDPattern.MatchString(strings.TrimSpace(owner)) || ttl < 5*time.Second || ttl > 10*time.Minute {
		return fmt.Errorf("cutover controller ownership request is invalid")
	}
	return nil
}
