package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
	corecontact "github.com/JekYUlll/Dipole/internal/services/core/domain/contact"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
	coregroup "github.com/JekYUlll/Dipole/internal/services/core/domain/group"
	coresession "github.com/JekYUlll/Dipole/internal/services/core/domain/session"
)

func TestDecodeGroupEventPayloadCompatibility(t *testing.T) {
	t.Parallel()

	legacy, err := coregroup.DecodeEventPayload("group.dismissed", json.RawMessage(`{"group_uuid":"G1","group_name":"Legacy","future_field":true}`))
	if err != nil {
		t.Fatalf("decode legacy Group event: %v", err)
	}
	if legacy.GroupUUID != "G1" || legacy.GroupName != "Legacy" {
		t.Fatalf("unexpected legacy Group payload: %+v", legacy)
	}

	current, err := coregroup.DecodeEventPayload("group.members.added", json.RawMessage(`{"group_uuid":"G1","operator_uuid":"U1","member_uuids":["U2"]}`))
	if err != nil || len(current.MemberUUIDs) != 1 {
		t.Fatalf("decode current Group event: payload=%+v err=%v", current, err)
	}

	_, err = coregroup.DecodeEventPayload("group.owner.transferred", json.RawMessage(`{"group_uuid":"G1"}`))
	if !errors.Is(err, platformEvents.ErrUnsupportedType) {
		t.Fatalf("expected unsupported Group event, got %v", err)
	}
}

func TestDecodeConversationReadReceiptCompatibility(t *testing.T) {
	t.Parallel()

	legacy, err := coreconversation.DecodeReadReceipt("conversation.direct.read", json.RawMessage(`{
		"reader_uuid":"U1","target_uuid":"U2","target_type":0,
		"conversation_key":"direct:U1:U2","last_read_message_uuid":"M1"
	}`))
	if err != nil {
		t.Fatalf("decode legacy read receipt: %v", err)
	}
	if legacy.LastReadSeq != 0 || legacy.TargetType != model.MessageTargetDirect {
		t.Fatalf("unexpected legacy read receipt: %+v", legacy)
	}

	_, err = coreconversation.DecodeReadReceipt("conversation.direct.read", json.RawMessage(`{"target_type":1}`))
	if !errors.Is(err, coreconversation.ErrReadReceiptTargetMismatch) {
		t.Fatalf("expected read target mismatch, got %v", err)
	}
}

func TestDecodeContactAndSessionEventPayloads(t *testing.T) {
	t.Parallel()

	contact, err := corecontact.DecodeFriendDeletedPayload("contact.friend.deleted", json.RawMessage(`{"user_uuid":"U1","friend_uuid":"U2","future_field":"ok"}`))
	if err != nil || contact.UserUUID != "U1" || contact.FriendUUID != "U2" {
		t.Fatalf("decode Contact event: payload=%+v err=%v", contact, err)
	}

	session, err := coresession.DecodeKickEventPayload("session.force_logout", json.RawMessage(`{"user_uuid":"U1","all":true,"reason":"forced"}`))
	if err != nil || !session.All || session.Reason != "forced" {
		t.Fatalf("decode Session event: payload=%+v err=%v", session, err)
	}

	_, err = coresession.DecodeKickEventPayload("session.connected", json.RawMessage(`{"user_uuid":"U1"}`))
	if !errors.Is(err, platformEvents.ErrUnsupportedType) {
		t.Fatalf("expected unsupported Session event, got %v", err)
	}
}

func TestDomainEventDecodersRejectMalformedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		decode func() error
	}{
		{name: "group", decode: func() error {
			_, err := coregroup.DecodeEventPayload("group.created", json.RawMessage(`{`))
			return err
		}},
		{name: "conversation", decode: func() error {
			_, err := coreconversation.DecodeReadReceipt("conversation.direct.read", json.RawMessage(`{`))
			return err
		}},
		{name: "contact", decode: func() error {
			_, err := corecontact.DecodeFriendDeletedPayload("contact.friend.deleted", json.RawMessage(`{`))
			return err
		}},
		{name: "session", decode: func() error {
			_, err := coresession.DecodeKickEventPayload("session.force_logout", json.RawMessage(`{`))
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.decode(); err == nil {
				t.Fatal("expected malformed payload to fail")
			}
		})
	}
}
