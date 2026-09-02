package agentmysql

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestTimelineEventGeneratedUUIDFitsSchemaForMaximumTaskUUID(t *testing.T) {
	event := timelineEvent(strings.Repeat("T", 64), strings.Repeat("R", 64), application.AgentTaskTimelineEventTask, "running")
	if len(event.EventUUID) != 64 {
		t.Fatalf("event UUID length=%d, want 64", len(event.EventUUID))
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("generated event is invalid: %v", err)
	}
}
