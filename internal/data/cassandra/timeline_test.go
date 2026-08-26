package cassandra

import (
	"testing"
	"time"
)

func TestBucketForSequenceUsesStableInclusiveBoundaries(t *testing.T) {
	tests := []struct {
		sequence uint64
		want     int64
	}{
		{sequence: 1, want: 0},
		{sequence: 10_000, want: 0},
		{sequence: 10_001, want: 1},
		{sequence: 20_000, want: 1},
	}
	for _, tt := range tests {
		bucket, err := BucketForSequence(tt.sequence, DefaultTimelineBucketSize)
		if err != nil {
			t.Fatalf("bucket sequence %d: %v", tt.sequence, err)
		}
		if bucket != tt.want {
			t.Fatalf("sequence %d: expected bucket %d, got %d", tt.sequence, tt.want, bucket)
		}
	}
}

func TestBucketForSequenceRejectsInvalidInputs(t *testing.T) {
	if _, err := BucketForSequence(0, DefaultTimelineBucketSize); err == nil {
		t.Fatal("expected zero sequence to fail")
	}
	if _, err := BucketForSequence(1, 0); err == nil {
		t.Fatal("expected zero bucket size to fail")
	}
}

func TestTimelineProjectionHashIgnoresEnvelopeIdentity(t *testing.T) {
	projection := validProjection()
	first, err := projection.PayloadHash()
	if err != nil {
		t.Fatalf("hash first projection: %v", err)
	}
	projection.EventID = "E2"
	projection.EventVersion = "v1.1"
	second, err := projection.PayloadHash()
	if err != nil {
		t.Fatalf("hash replay projection: %v", err)
	}
	if first != second {
		t.Fatalf("expected envelope identity to be excluded: %s != %s", first, second)
	}

	projection.Content = "changed"
	changed, err := projection.PayloadHash()
	if err != nil {
		t.Fatalf("hash changed projection: %v", err)
	}
	if changed == first {
		t.Fatal("expected payload changes to alter projection hash")
	}
}

func validProjection() TimelineProjection {
	return TimelineProjection{
		EventID:         "E1",
		EventVersion:    "v1",
		ConversationKey: "direct:U1:U2",
		MessageSeq:      1,
		MessageUUID:     "M1",
		ClientMessageID: "C1",
		SenderUUID:      "U1",
		TargetUUID:      "U2",
		Content:         "hello",
		SentAt:          time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
	}
}
