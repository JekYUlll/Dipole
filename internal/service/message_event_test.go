package service

import (
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestNormalizeMessageMutationContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType string
		payload   MessageEventPayload
		wantType  MessageMutationType
		wantRev   uint64
		wantActor string
		wantErr   error
	}{
		{
			name:      "legacy created defaults",
			eventType: "message.direct.created",
			payload:   MessageEventPayload{SenderUUID: "U1"},
			wantType:  MessageMutationCreated,
			wantRev:   1,
			wantActor: "U1",
		},
		{
			name:      "explicit created",
			eventType: "message.group.created",
			payload: MessageEventPayload{
				MutationType: MessageMutationCreated,
				Revision:     1,
				ActorUUID:    "U2",
			},
			wantType:  MessageMutationCreated,
			wantRev:   1,
			wantActor: "U2",
		},
		{
			name:      "future edited contract",
			eventType: "message.direct.edited",
			payload: MessageEventPayload{
				MutationType: MessageMutationEdited,
				Revision:     2,
				ActorUUID:    "U1",
			},
			wantType:  MessageMutationEdited,
			wantRev:   2,
			wantActor: "U1",
		},
		{
			name:      "event and payload disagree",
			eventType: "message.direct.created",
			payload: MessageEventPayload{
				MutationType: MessageMutationDeleted,
				Revision:     1,
				ActorUUID:    "U1",
			},
			wantErr: ErrMessageMutationTypeMismatch,
		},
		{
			name:      "created revision must be one",
			eventType: "message.direct.created",
			payload: MessageEventPayload{
				MutationType: MessageMutationCreated,
				Revision:     2,
				ActorUUID:    "U1",
			},
			wantErr: ErrMessageMutationRevisionInvalid,
		},
		{
			name:      "future mutation requires revision",
			eventType: "message.group.recalled",
			payload: MessageEventPayload{
				MutationType: MessageMutationRecalled,
				ActorUUID:    "U1",
			},
			wantErr: ErrMessageMutationRevisionRequired,
		},
		{
			name:      "future mutation requires actor",
			eventType: "message.group.deleted",
			payload: MessageEventPayload{
				MutationType: MessageMutationDeleted,
				Revision:     2,
			},
			wantErr: ErrMessageMutationActorRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := test.payload
			err := NormalizeMessageMutation(test.eventType, &payload)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("normalize mutation error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if payload.MutationType != test.wantType || payload.Revision != test.wantRev || payload.ActorUUID != test.wantActor {
				t.Fatalf("unexpected normalized payload: %+v", payload)
			}
		})
	}
}

func TestMessageMutationEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		targetType int8
		mutation   MessageMutationType
		want       string
	}{
		{model.MessageTargetDirect, MessageMutationCreated, "message.direct.created"},
		{model.MessageTargetGroup, MessageMutationCreated, "message.group.created"},
		{model.MessageTargetDirect, MessageMutationEdited, "message.direct.edited"},
		{model.MessageTargetGroup, MessageMutationRecalled, "message.group.recalled"},
		{model.MessageTargetDirect, MessageMutationDeleted, "message.direct.deleted"},
	}

	for _, test := range tests {
		got, err := MessageMutationEventType(test.targetType, test.mutation)
		if err != nil || got != test.want {
			t.Fatalf("event type (%d, %q) = %q, %v; want %q", test.targetType, test.mutation, got, err, test.want)
		}
	}
}

func TestMessageMutationAggregateIDPreservesCreatedCompatibility(t *testing.T) {
	t.Parallel()

	created, err := MessageMutationAggregateID("M1", MessageMutationCreated, 1)
	if err != nil || created != "M1" {
		t.Fatalf("created aggregate id = %q, %v; want M1", created, err)
	}
	edited2, err := MessageMutationAggregateID("M1", MessageMutationEdited, 2)
	if err != nil || edited2 != "M1@r2" {
		t.Fatalf("edited revision 2 aggregate id = %q, %v", edited2, err)
	}
	edited3, err := MessageMutationAggregateID("M1", MessageMutationEdited, 3)
	if err != nil || edited3 == edited2 {
		t.Fatalf("edited revision 3 aggregate id = %q, %v; must differ from %q", edited3, err, edited2)
	}
}
