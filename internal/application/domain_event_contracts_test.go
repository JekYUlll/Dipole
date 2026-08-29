package application

import (
	"encoding/json"
	"testing"
)

func TestDomainEventContractsDecodeAndValidate(t *testing.T) {
	group, err := DecodeGroupEventPayload("group.updated", json.RawMessage(`{"group_uuid":"G1","recipient_uuids":["U1"]}`))
	if err != nil || group.GroupUUID != "G1" || len(group.RecipientUUIDs) != 1 {
		t.Fatalf("decode group event: payload=%+v err=%v", group, err)
	}

	read, err := DecodeConversationReadReceipt("conversation.direct.read", json.RawMessage(`{"target_uuid":"U2","target_type":0,"last_read_seq":9}`))
	if err != nil || read.TargetUUID != "U2" || read.LastReadSeq != 9 {
		t.Fatalf("decode read receipt: payload=%+v err=%v", read, err)
	}
	if _, err := DecodeConversationReadReceipt("conversation.direct.read", json.RawMessage(`{"target_type":1}`)); err == nil {
		t.Fatal("group read receipt must be rejected by the direct event contract")
	}

	if _, err := DecodeSessionKickEventPayload("session.connected", json.RawMessage(`{"user_uuid":"U1"}`)); err == nil {
		t.Fatal("unsupported session event must be rejected")
	}
	contact, err := DecodeContactFriendDeletedPayload("contact.friend.deleted", json.RawMessage(`{"user_uuid":"U1","friend_uuid":"U2"}`))
	if err != nil || contact.FriendUUID != "U2" {
		t.Fatalf("decode contact event: payload=%+v err=%v", contact, err)
	}

}

func TestDomainEventContractsRejectMalformedPayload(t *testing.T) {
	decoders := []struct {
		eventType string
		decode    func(string, json.RawMessage) error
	}{
		{eventType: "group.updated", decode: func(eventType string, raw json.RawMessage) error {
			_, err := DecodeGroupEventPayload(eventType, raw)
			return err
		}},
		{eventType: "session.force_logout", decode: func(eventType string, raw json.RawMessage) error {
			_, err := DecodeSessionKickEventPayload(eventType, raw)
			return err
		}},
		{eventType: "contact.friend.deleted", decode: func(eventType string, raw json.RawMessage) error {
			_, err := DecodeContactFriendDeletedPayload(eventType, raw)
			return err
		}},
		{eventType: "conversation.direct.read", decode: func(eventType string, raw json.RawMessage) error {
			_, err := DecodeConversationReadReceipt(eventType, raw)
			return err
		}},
	}
	for _, decoder := range decoders {
		if err := decoder.decode(decoder.eventType, json.RawMessage(`{"broken":`)); err == nil {
			t.Fatal("malformed payload must be rejected")
		}
	}
}
