package model

import "time"

type DeviceSyncCheckpoint struct {
	UserUUID  string    `json:"user_uuid"`
	DeviceID  string    `json:"device_id"`
	SyncSeq   uint64    `json:"sync_seq"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
