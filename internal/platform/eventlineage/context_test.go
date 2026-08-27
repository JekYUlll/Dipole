package eventlineage

import (
	"context"
	"testing"
)

func TestContextCarriesNormalizedAgentLineage(t *testing.T) {
	t.Parallel()

	ctx := WithContext(context.Background(), Lineage{
		Origin:           Origin{Type: OriginAgent, ID: " UAI "},
		CausationEventID: " E1 ",
		AgentTaskID:      " TASK-1 ",
	})

	got := FromContext(ctx)
	want := Lineage{
		Origin:           Origin{Type: OriginAgent, ID: "UAI"},
		CausationEventID: "E1",
		AgentTaskID:      "TASK-1",
	}
	if got != want {
		t.Fatalf("lineage = %+v, want %+v", got, want)
	}
}

func TestAdvancePreservesOriginAndTaskWhileRollingCausation(t *testing.T) {
	t.Parallel()

	got := Advance(Lineage{
		Origin:           Origin{Type: OriginAgent, ID: "UAI"},
		CausationEventID: "E0",
		AgentTaskID:      "TASK-1",
	}, "E1")
	if got.Origin != (Origin{Type: OriginAgent, ID: "UAI"}) || got.AgentTaskID != "TASK-1" || got.CausationEventID != "E1" {
		t.Fatalf("unexpected advanced lineage: %+v", got)
	}
}

func TestValidateRequiresTaskForAgentOrigin(t *testing.T) {
	t.Parallel()

	err := Validate(Lineage{Origin: Origin{Type: OriginAgent, ID: "UAI"}, CausationEventID: "E1"})
	if err == nil {
		t.Fatal("expected agent lineage without task to be rejected")
	}
}

func TestAgentActionPreservesExistingAgentRoot(t *testing.T) {
	t.Parallel()

	ctx := WithContext(context.Background(), Lineage{
		Origin:           Origin{Type: OriginAgent, ID: "AGENT-A"},
		CausationEventID: "E1",
		AgentTaskID:      "TASK-A",
	})
	got := FromContext(AgentAction(ctx, "AGENT-B", "TASK-B", "E2"))
	if got.Origin != (Origin{Type: OriginAgent, ID: "AGENT-A"}) || got.AgentTaskID != "TASK-A" || got.CausationEventID != "E2" {
		t.Fatalf("Agent root was not preserved: %+v", got)
	}
}
