package bootstrap

import (
	"errors"
	"testing"
)

func TestQueryOnlyMessageApplicationRejectsEveryCommand(t *testing.T) {
	application := newQueryOnlyMessageApplication(stubMessageApplication{})
	if _, err := application.SendDirectMessage("U1", "U2", "text", "C1"); !errors.Is(err, errMessageShadowCommand) {
		t.Fatalf("direct command error = %v", err)
	}
	if _, _, err := application.SendGroupMessage("U1", "G1", "text", "C2"); !errors.Is(err, errMessageShadowCommand) {
		t.Fatalf("group command error = %v", err)
	}
	if _, err := application.SendDirectFileMessage("U1", "U2", "F1", "C3"); !errors.Is(err, errMessageShadowCommand) {
		t.Fatalf("direct file command error = %v", err)
	}
	if _, _, err := application.SendGroupFileMessage("U1", "G1", "F1", "C4"); !errors.Is(err, errMessageShadowCommand) {
		t.Fatalf("group file command error = %v", err)
	}
	if messages, err := application.ListDirectMessages("U1", "U2", 40, 20); err != nil || len(messages) != 1 {
		t.Fatalf("query should pass through: messages=%+v err=%v", messages, err)
	}
}
