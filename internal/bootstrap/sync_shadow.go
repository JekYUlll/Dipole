package bootstrap

import (
	"reflect"
	"sync"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"go.uber.org/zap"
)

type syncShadowComparison struct {
	Operation string
	Outcome   string
}

type syncShadowApplication struct {
	primary application.SyncApplication
	shadow  application.SyncApplication
	observe func(syncShadowComparison)
	wait    sync.WaitGroup
}

func newSyncShadowApplication(primary, shadow application.SyncApplication, observe func(syncShadowComparison)) *syncShadowApplication {
	if observe == nil {
		observe = func(comparison syncShadowComparison) {
			logger.Debug("Sync shadow query compared", zap.String("operation", comparison.Operation), zap.String("outcome", comparison.Outcome))
		}
	}
	return &syncShadowApplication{primary: primary, shadow: shadow, observe: observe}
}

func (s *syncShadowApplication) List(user string, after uint64, limit int) (*application.SyncPage, error) {
	result, err := s.primary.List(user, after, limit)
	s.compare("list", result, err, func() (any, error) { return s.shadow.List(user, after, limit) })
	return result, err
}

func (s *syncShadowApplication) GetCheckpoint(user, device string) (*model.DeviceSyncCheckpoint, error) {
	result, err := s.primary.GetCheckpoint(user, device)
	s.compare("get_checkpoint", result, err, func() (any, error) { return s.shadow.GetCheckpoint(user, device) })
	return result, err
}

func (s *syncShadowApplication) AdvanceCheckpoint(user, device string, sequence uint64) (*model.DeviceSyncCheckpoint, error) {
	return s.primary.AdvanceCheckpoint(user, device, sequence)
}

func (s *syncShadowApplication) ListGroupCheckpoints(user, device string, groups []string) ([]*model.GroupSyncCheckpoint, error) {
	result, err := s.primary.ListGroupCheckpoints(user, device, groups)
	s.compare("list_group_checkpoints", result, err, func() (any, error) { return s.shadow.ListGroupCheckpoints(user, device, groups) })
	return result, err
}

func (s *syncShadowApplication) AdvanceGroupCheckpoint(user, device, group string, sequence uint64) (*model.GroupSyncCheckpoint, error) {
	return s.primary.AdvanceGroupCheckpoint(user, device, group, sequence)
}

func (s *syncShadowApplication) compare(operation string, primary any, primaryErr error, query func() (any, error)) {
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		shadow, shadowErr := query()
		outcome := "match"
		if (primaryErr == nil) != (shadowErr == nil) {
			outcome = "error_mismatch"
		} else if primaryErr == nil && !reflect.DeepEqual(primary, shadow) {
			outcome = "payload_mismatch"
		}
		s.observe(syncShadowComparison{Operation: operation, Outcome: outcome})
	}()
}

func (s *syncShadowApplication) Wait() {
	if s != nil {
		s.wait.Wait()
	}
}
