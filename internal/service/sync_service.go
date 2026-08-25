package service

import (
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

type syncRepository interface {
	ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error)
}

type SyncPage struct {
	Items   []*model.SyncMessage
	NextSeq uint64
	HasMore bool
}

type SyncService struct {
	repo syncRepository
}

func NewSyncService(repo syncRepository) *SyncService {
	return &SyncService{repo: repo}
}

func (s *SyncService) List(userUUID string, afterSeq uint64, limit int) (*SyncPage, error) {
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
	return &SyncPage{Items: items, NextSeq: nextSeq, HasMore: hasMore}, nil
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
