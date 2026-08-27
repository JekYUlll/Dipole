package model

import "time"

type GroupSyncState struct {
	GroupUUID         string
	LatestMessageSeq  uint64
	LatestMessageUUID string
	UpdatedAt         time.Time
}

type GroupSyncCheckpoint struct {
	GroupUUID         string
	LatestMessageSeq  uint64
	LatestMessageUUID string
	PulledMessageSeq  uint64
}
