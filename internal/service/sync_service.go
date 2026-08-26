package service

import (
	"errors"
	"fmt"
	"strings"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrSyncDeviceIDRequired = errors.New("sync device id is required")
	ErrSyncDeviceIDInvalid  = errors.New("sync device id is invalid")
	ErrSyncCheckpointAhead  = errors.New("sync checkpoint exceeds available timeline")
)

type syncRepository interface {
	ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error)
	GetDeviceCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error)
	GetLatestUserSyncSequence(userUUID string) (uint64, error)
	AdvanceDeviceSyncCheckpoint(userUUID, deviceID string, syncSeq uint64) error
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
	repo syncRepository
}

func NewSyncService(repo syncRepository) *SyncService {
	return &SyncService{repo: repo}
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
