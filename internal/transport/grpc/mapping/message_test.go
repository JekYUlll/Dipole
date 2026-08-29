package grpcmapping

import (
	"testing"

	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestMessageConversationSequenceRoundTrip(t *testing.T) {
	original := &model.Message{UUID: "M1", ConversationKey: "group:G1", Seq: 42}
	decoded := MessageFromProto(MessageToProto(original))
	if decoded == nil || decoded.Seq != 42 {
		t.Fatalf("message sequence round trip = %+v", decoded)
	}
}

func TestMessageFromLegacyProtoDefaultsConversationSequence(t *testing.T) {
	decoded := MessageFromProto(&messagev1.Message{ServerMessageId: "M-legacy"})
	if decoded == nil || decoded.Seq != 0 {
		t.Fatalf("legacy message sequence = %+v, want zero", decoded)
	}
}
