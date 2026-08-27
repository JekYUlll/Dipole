package grpcmapping

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
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
