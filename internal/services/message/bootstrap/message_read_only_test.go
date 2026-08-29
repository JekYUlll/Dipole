package bootstrap

import (
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestQueryOnlyMessageApplicationRejectsEveryCommand(t *testing.T) {
	application := newQueryOnlyMessageApplication(messageQueryStub{})
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

type messageQueryStub struct{}

func (messageQueryStub) ListDirectMessages(string, string, uint, int) ([]*model.Message, error) {
	return []*model.Message{{UUID: "M1"}}, nil
}

func (messageQueryStub) ListDirectMessagesBeforeSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListDirectMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListGroupMessages(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListGroupMessagesBeforeSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListGroupMessagesAfter(string, string, uint, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListGroupMessagesAfterSeq(string, string, uint64, int) ([]*model.Message, error) {
	return nil, nil
}

func (messageQueryStub) ListOfflineMessages(string, uint, int) ([]*model.Message, error) {
	return nil, nil
}
