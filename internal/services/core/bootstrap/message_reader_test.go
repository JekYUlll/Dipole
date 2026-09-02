package bootstrap

import (
	"context"
	"io"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
)

type messageHistoryClientStub struct {
	directCalls int
	groupCalls  int
}

func (s *messageHistoryClientStub) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	s.directCalls++
	return []*model.Message{{UUID: "M-direct", SenderUUID: currentUserUUID, TargetUUID: targetUUID, ID: beforeID + uint(limit)}}, nil
}

func (s *messageHistoryClientStub) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	s.groupCalls++
	return []*model.Message{{UUID: "M-group", SenderUUID: currentUserUUID, TargetUUID: groupUUID, ID: beforeID + uint(limit)}}, nil
}

type closerStub struct{ calls int }

func (s *closerStub) Close() error {
	s.calls++
	return nil
}

func TestLazyCoreMessageReaderDialsOnceAndClosesItsRemoteConnection(t *testing.T) {
	client := &messageHistoryClientStub{}
	conn := &closerStub{}
	dials := 0
	reader := &lazyCoreMessageReader{
		cfg: config.InternalRPC{},
		dial: func(context.Context, config.InternalRPC) (coreMessageHistoryClient, io.Closer, error) {
			dials++
			return client, conn, nil
		},
	}

	direct, err := reader.ListDirectMessages("U100", "U200", 4, 20)
	if err != nil || len(direct) != 1 || direct[0].UUID != "M-direct" {
		t.Fatalf("ListDirectMessages() = %#v, %v", direct, err)
	}
	group, err := reader.ListGroupMessages("U100", "G200", 2, 20)
	if err != nil || len(group) != 1 || group[0].UUID != "M-group" {
		t.Fatalf("ListGroupMessages() = %#v, %v", group, err)
	}
	if dials != 1 || client.directCalls != 1 || client.groupCalls != 1 {
		t.Fatalf("dials=%d direct=%d group=%d, want 1 each", dials, client.directCalls, client.groupCalls)
	}
	if err := reader.Close(); err != nil || conn.calls != 1 {
		t.Fatalf("Close() = %v, calls=%d", err, conn.calls)
	}
}
