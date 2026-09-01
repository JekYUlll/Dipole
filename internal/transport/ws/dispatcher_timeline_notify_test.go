package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestDispatcherPrimaryTimelineNotificationIsLocatorOnly(t *testing.T) {
	hub := NewHub()
	recipient := &Client{sessionUser: &SessionUser{UUID: "U200"}, send: make(chan []byte, 2)}
	hub.Register(recipient)
	sender := &Client{sessionUser: &SessionUser{UUID: "U100"}, send: make(chan []byte, 1)}
	dispatcher := NewDispatcher(hub, nil, nil, true).WithTimelineNotifyMode(TimelineNotifyPrimary)

	dispatcher.dispatchDirect(context.Background(), sender, &model.Message{
		UUID: "M100", ConversationKey: "direct:U100:U200", Seq: 42,
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "message body", SentAt: time.Now().UTC(),
	})

	select {
	case payload := <-recipient.send:
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode recipient event: %v", err)
		}
		if event.Type != TypeSyncItemNotifyV1 {
			t.Fatalf("recipient event type=%s want=%s", event.Type, TypeSyncItemNotifyV1)
		}
		var notify SyncItemNotifyData
		if err := json.Unmarshal(event.Data, &notify); err != nil {
			t.Fatalf("decode timeline notification: %v", err)
		}
		if notify.MessageUUID != "M100" || notify.MessageSeq != 42 || notify.ConversationKey != "direct:U100:U200" {
			t.Fatalf("unexpected timeline locator: %+v", notify)
		}
	default:
		t.Fatal("recipient did not receive the primary timeline notification")
	}
	if len(recipient.send) != 0 {
		t.Fatal("primary recipient received an unexpected full-message event")
	}

	select {
	case payload := <-sender.send:
		var event struct {
			Type string       `json:"type"`
			Data ChatSentData `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode sender acknowledgement: %v", err)
		}
		if event.Type != TypeChatSent || !event.Data.Delivered {
			t.Fatalf("unexpected sender acknowledgement: %+v", event)
		}
	default:
		t.Fatal("sender did not receive a delivery acknowledgement")
	}
}
