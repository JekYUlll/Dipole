package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/service"
)

type DeviceSessionResponse struct {
	ConnectionID string `json:"connection_id"`
	Device       string `json:"device"`
	DeviceID     string `json:"device_id,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	RemoteAddr   string `json:"remote_addr,omitempty"`
	NodeID       string `json:"node_id"`
	ConnectedAt  string `json:"connected_at"`
	LastSeenAt   string `json:"last_seen_at"`
}

func ToDeviceSessionResponses(devices []*service.DeviceSessionView) []*DeviceSessionResponse {
	result := make([]*DeviceSessionResponse, 0, len(devices))
	for _, device := range devices {
		if device == nil {
			continue
		}
		result = append(result, &DeviceSessionResponse{
			ConnectionID: device.ConnectionID,
			Device:       device.Device,
			DeviceID:     device.DeviceID,
			UserAgent:    device.UserAgent,
			RemoteAddr:   device.RemoteAddr,
			NodeID:       device.NodeID,
			ConnectedAt:  device.ConnectedAt.Format(time.RFC3339),
			LastSeenAt:   device.LastSeenAt.Format(time.RFC3339),
		})
	}

	return result
}
