package httpdto

import (
	"time"

	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

type DeviceSessionResponse struct {
	ConnectionID string `json:"connection_id"`
	Device       string `json:"device"`
	DeviceID     string `json:"device_id,omitempty"`
	ConnectedAt  string `json:"connected_at"`
	LastSeenAt   string `json:"last_seen_at"`
}

func ToDeviceSessionResponses(devices []*coresession.DeviceSessionView) []*DeviceSessionResponse {
	result := make([]*DeviceSessionResponse, 0, len(devices))
	for _, device := range devices {
		if device == nil {
			continue
		}
		result = append(result, &DeviceSessionResponse{
			ConnectionID: device.ConnectionID,
			Device:       device.Device,
			DeviceID:     device.DeviceID,
			ConnectedAt:  device.ConnectedAt.Format(time.RFC3339),
			LastSeenAt:   device.LastSeenAt.Format(time.RFC3339),
		})
	}

	return result
}
