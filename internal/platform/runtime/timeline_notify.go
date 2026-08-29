package runtime

import "fmt"

const (
	timelineNotifyOff     = "off"
	timelineNotifyShadow  = "shadow"
	timelineNotifyPrimary = "primary"
)

// ValidateTimelineNotifyMode validates the rollout mode for body-free
// timeline notifications shared by the gateway and embedded runtimes.
func ValidateTimelineNotifyMode(mode string) error {
	if mode != timelineNotifyOff && mode != timelineNotifyShadow && mode != timelineNotifyPrimary {
		return fmt.Errorf("unsupported message.timeline_notify_mode %q", mode)
	}
	return nil
}
