package syncbaseline

import (
	"testing"
)

func TestDigestIsOrderIndependentAndRejectsDuplicateSyncSequence(t *testing.T) {
	entries := []Entry{
		{SyncSeq: 9, UserUUID: "U2", MessageUUID: "M2", ConversationKey: "group:G1", MessageSeq: 4},
		{SyncSeq: 3, UserUUID: "U1", MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 2},
	}
	digest, canonical, err := Digest(entries)
	if err != nil {
		t.Fatalf("digest baseline: %v", err)
	}
	if digest == "" || len(digest) != 64 {
		t.Fatalf("unexpected digest %q", digest)
	}
	if canonical[0].SyncSeq != 3 || canonical[1].SyncSeq != 9 {
		t.Fatalf("entries are not canonical: %+v", canonical)
	}
	reversedDigest, _, err := Digest([]Entry{entries[1], entries[0]})
	if err != nil || reversedDigest != digest {
		t.Fatalf("digest must be deterministic: digest=%q reversed=%q err=%v", digest, reversedDigest, err)
	}
	if _, _, err := Digest([]Entry{entries[0], entries[0]}); err == nil {
		t.Fatal("expected duplicate sync sequence to fail")
	}
}

func TestCompareReportsMissingExtraAndConflictingRows(t *testing.T) {
	expected := []Entry{
		{SyncSeq: 1, UserUUID: "U1", MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1},
		{SyncSeq: 2, UserUUID: "U2", MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1},
		{SyncSeq: 3, UserUUID: "U3", MessageUUID: "M2", ConversationKey: "group:G1", MessageSeq: 7},
	}
	actual := []Entry{
		expected[0],
		{SyncSeq: 8, UserUUID: "U2", MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1},
		{SyncSeq: 4, UserUUID: "U4", MessageUUID: "M3", ConversationKey: "group:G2", MessageSeq: 1},
	}
	report, err := Compare("baseline-v1", 10, expected, actual, 10)
	if err != nil {
		t.Fatalf("compare baseline: %v", err)
	}
	if report.Consistent || report.Missing != 1 || report.Extra != 1 || report.Conflicting != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Examples) != 3 {
		t.Fatalf("unexpected examples: %+v", report.Examples)
	}
}

func TestCompareRejectsInvalidBounds(t *testing.T) {
	_, err := Compare("baseline-v1", 2, []Entry{{
		SyncSeq: 3, UserUUID: "U1", MessageUUID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1,
	}}, nil, 10)
	if err == nil {
		t.Fatal("expected entry above fixed high watermark to fail")
	}
}
