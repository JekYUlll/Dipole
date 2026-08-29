package syncdomain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrSyncDeviceIDRequired = errors.New("sync device id is required")
	ErrSyncDeviceIDInvalid  = errors.New("sync device id is invalid")
	ErrSyncCheckpointAhead  = errors.New("sync checkpoint exceeds available timeline")
	ErrSyncGroupRequired    = errors.New("sync group id is required")
	ErrSyncGroupForbidden   = errors.New("sync group is forbidden")
	ErrSyncGroupLimit       = errors.New("too many sync groups requested")
)

type syncRepository interface {
	ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error)
	GetDeviceCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error)
	GetLatestUserSyncSequence(userUUID string) (uint64, error)
	AdvanceDeviceSyncCheckpoint(userUUID, deviceID string, syncSeq uint64) error
	ListGroupSyncCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error)
	GetGroupSyncState(groupUUID string) (*model.GroupSyncState, error)
	AdvanceDeviceGroupSyncCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) error
}

// SyncGroupAuthorizer authorizes access to group sync timelines.
type SyncGroupAuthorizer interface {
	GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error)
}

func (s *SyncService) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	userUUID = strings.TrimSpace(userUUID)
	deviceID, err := normalizeSyncDeviceID(deviceID)
	if err != nil {
		return nil, err
	}
	checkpoint, err := s.repo.GetDeviceCheckpoint(userUUID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("get device sync checkpoint: %w", err)
	}
	if checkpoint == nil {
		checkpoint = &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID}
	}
	return checkpoint, nil
}

func (s *SyncService) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	userUUID = strings.TrimSpace(userUUID)
	deviceID, err := normalizeSyncDeviceID(deviceID)
	if err != nil {
		return nil, err
	}
	latest, err := s.repo.GetLatestUserSyncSequence(userUUID)
	if err != nil {
		return nil, fmt.Errorf("get latest user sync sequence: %w", err)
	}
	if syncSeq > latest {
		return nil, ErrSyncCheckpointAhead
	}
	if err := s.repo.AdvanceDeviceSyncCheckpoint(userUUID, deviceID, syncSeq); err != nil {
		return nil, fmt.Errorf("advance device sync checkpoint: %w", err)
	}
	return s.GetCheckpoint(userUUID, deviceID)
}

func normalizeSyncDeviceID(deviceID string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", ErrSyncDeviceIDRequired
	}
	if len(deviceID) > 128 {
		return "", ErrSyncDeviceIDInvalid
	}
	return deviceID, nil
}

type SyncService struct {
	repo       syncRepository
	groupsAuth SyncGroupAuthorizer
}

func NewSyncService(repo syncRepository, authorizers ...SyncGroupAuthorizer) *SyncService {
	service := &SyncService{repo: repo}
	if len(authorizers) > 0 {
		service.groupsAuth = authorizers[0]
	}
	return service
}

func (s *SyncService) ListGroupCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	userUUID = strings.TrimSpace(userUUID)
	deviceID, err := normalizeSyncDeviceID(deviceID)
	if err != nil {
		return nil, err
	}
	groupUUIDs, err = normalizeSyncGroupUUIDs(groupUUIDs)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSyncGroups(userUUID, groupUUIDs); err != nil {
		return nil, err
	}
	stored, err := s.repo.ListGroupSyncCheckpoints(userUUID, deviceID, groupUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list group sync checkpoints: %w", err)
	}
	byGroup := make(map[string]*model.GroupSyncCheckpoint, len(stored))
	for _, checkpoint := range stored {
		if checkpoint != nil {
			byGroup[checkpoint.GroupUUID] = checkpoint
		}
	}
	result := make([]*model.GroupSyncCheckpoint, 0, len(groupUUIDs))
	for _, groupUUID := range groupUUIDs {
		checkpoint := byGroup[groupUUID]
		if checkpoint == nil {
			checkpoint = &model.GroupSyncCheckpoint{GroupUUID: groupUUID}
		}
		result = append(result, checkpoint)
	}
	return result, nil
}

func (s *SyncService) AdvanceGroupCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, ErrSyncGroupRequired
	}
	if _, err := normalizeSyncDeviceID(deviceID); err != nil {
		return nil, err
	}
	if err := s.authorizeSyncGroups(strings.TrimSpace(userUUID), []string{groupUUID}); err != nil {
		return nil, err
	}
	state, err := s.repo.GetGroupSyncState(groupUUID)
	if err != nil {
		return nil, fmt.Errorf("get group sync state: %w", err)
	}
	latest := uint64(0)
	if state != nil {
		latest = state.LatestMessageSeq
	}
	if messageSeq > latest {
		return nil, ErrSyncCheckpointAhead
	}
	if err := s.repo.AdvanceDeviceGroupSyncCheckpoint(strings.TrimSpace(userUUID), strings.TrimSpace(deviceID), groupUUID, messageSeq); err != nil {
		return nil, fmt.Errorf("advance device group sync checkpoint: %w", err)
	}
	checkpoints, err := s.ListGroupCheckpoints(userUUID, deviceID, []string{groupUUID})
	if err != nil {
		return nil, err
	}
	return checkpoints[0], nil
}

func (s *SyncService) authorizeSyncGroups(userUUID string, groupUUIDs []string) error {
	if s.groupsAuth == nil {
		return ErrSyncGroupForbidden
	}
	for _, groupUUID := range groupUUIDs {
		member, err := s.groupsAuth.GetGroupMember(groupUUID, userUUID)
		if err != nil {
			return fmt.Errorf("authorize group sync checkpoint: %w", err)
		}
		if member == nil {
			return ErrSyncGroupForbidden
		}
	}
	return nil
}

func normalizeSyncGroupUUIDs(groupUUIDs []string) ([]string, error) {
	if len(groupUUIDs) == 0 {
		return nil, ErrSyncGroupRequired
	}
	if len(groupUUIDs) > 100 {
		return nil, ErrSyncGroupLimit
	}
	seen := make(map[string]struct{}, len(groupUUIDs))
	result := make([]string, 0, len(groupUUIDs))
	for _, groupUUID := range groupUUIDs {
		groupUUID = strings.TrimSpace(groupUUID)
		if groupUUID == "" {
			return nil, ErrSyncGroupRequired
		}
		if _, ok := seen[groupUUID]; ok {
			continue
		}
		seen[groupUUID] = struct{}{}
		result = append(result, groupUUID)
	}
	sort.Strings(result)
	return result, nil
}

func (s *SyncService) List(userUUID string, afterSeq uint64, limit int) (*applicationPort.SyncPage, error) {
	limit = normalizeSyncListLimit(limit)
	items, err := s.repo.ListByUserAfter(strings.TrimSpace(userUUID), afterSeq, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list sync messages: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextSeq := afterSeq
	if len(items) > 0 {
		nextSeq = items[len(items)-1].SyncSeq
	}
	return &applicationPort.SyncPage{Items: items, NextSeq: nextSeq, HasMore: hasMore}, nil
}

func normalizeSyncListLimit(limit int) int {
	switch {
	case limit <= 0:
		return 100
	case limit > 200:
		return 200
	default:
		return limit
	}
}
