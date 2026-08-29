package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestDecodeGroupEventPayloadCompatibility(t *testing.T) {
	t.Parallel()

	legacy, err := DecodeGroupEventPayload("group.dismissed", json.RawMessage(`{"group_uuid":"G1","group_name":"Legacy","future_field":true}`))
	if err != nil {
		t.Fatalf("decode legacy Group event: %v", err)
	}
	if legacy.GroupUUID != "G1" || legacy.GroupName != "Legacy" {
		t.Fatalf("unexpected legacy Group payload: %+v", legacy)
	}

	current, err := DecodeGroupEventPayload("group.members.added", json.RawMessage(`{"group_uuid":"G1","operator_uuid":"U1","member_uuids":["U2"]}`))
	if err != nil || len(current.MemberUUIDs) != 1 {
		t.Fatalf("decode current Group event: payload=%+v err=%v", current, err)
	}

	_, err = DecodeGroupEventPayload("group.owner.transferred", json.RawMessage(`{"group_uuid":"G1"}`))
	if !errors.Is(err, ErrUnsupportedDomainEventType) {
		t.Fatalf("expected unsupported Group event, got %v", err)
	}
}

func TestDecodeConversationReadReceiptCompatibility(t *testing.T) {
	t.Parallel()

	legacy, err := DecodeConversationReadReceipt("conversation.direct.read", json.RawMessage(`{
		"reader_uuid":"U1","target_uuid":"U2","target_type":0,
		"conversation_key":"direct:U1:U2","last_read_message_uuid":"M1"
	}`))
	if err != nil {
		t.Fatalf("decode legacy read receipt: %v", err)
	}
	if legacy.LastReadSeq != 0 || legacy.TargetType != model.MessageTargetDirect {
		t.Fatalf("unexpected legacy read receipt: %+v", legacy)
	}

	_, err = DecodeConversationReadReceipt("conversation.direct.read", json.RawMessage(`{"target_type":1}`))
	if !errors.Is(err, ErrConversationReadTargetMismatch) {
		t.Fatalf("expected read target mismatch, got %v", err)
	}
}

func TestDecodeContactAndSessionEventPayloads(t *testing.T) {
	t.Parallel()

	contact, err := DecodeContactFriendDeletedPayload("contact.friend.deleted", json.RawMessage(`{"user_uuid":"U1","friend_uuid":"U2","future_field":"ok"}`))
	if err != nil || contact.UserUUID != "U1" || contact.FriendUUID != "U2" {
		t.Fatalf("decode Contact event: payload=%+v err=%v", contact, err)
	}

	session, err := DecodeSessionKickEventPayload("session.force_logout", json.RawMessage(`{"user_uuid":"U1","all":true,"reason":"forced"}`))
	if err != nil || !session.All || session.Reason != "forced" {
		t.Fatalf("decode Session event: payload=%+v err=%v", session, err)
	}

	_, err = DecodeSessionKickEventPayload("session.connected", json.RawMessage(`{"user_uuid":"U1"}`))
	if !errors.Is(err, ErrUnsupportedDomainEventType) {
		t.Fatalf("expected unsupported Session event, got %v", err)
	}
}

func TestDomainEventDecodersRejectMalformedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		decode func() error
	}{
		{name: "group", decode: func() error { _, err := DecodeGroupEventPayload("group.created", json.RawMessage(`{`)); return err }},
		{name: "conversation", decode: func() error {
			_, err := DecodeConversationReadReceipt("conversation.direct.read", json.RawMessage(`{`))
			return err
		}},
		{name: "contact", decode: func() error {
			_, err := DecodeContactFriendDeletedPayload("contact.friend.deleted", json.RawMessage(`{`))
			return err
		}},
		{name: "session", decode: func() error {
			_, err := DecodeSessionKickEventPayload("session.force_logout", json.RawMessage(`{`))
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
