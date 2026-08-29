package events

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedType = errors.New("unsupported domain event type")

// RequireType validates a versioned event against the consumer's allowlist.
func RequireType(actual string, allowed ...string) error {
	actual = strings.TrimSpace(actual)
	for _, eventType := range allowed {
		if actual == eventType {
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrUnsupportedType, actual)
}
