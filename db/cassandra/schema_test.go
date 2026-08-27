package cassandraschema

import (
	"strings"
	"testing"
)

func TestStatementsLoadsVersionedTimelineSchema(t *testing.T) {
	statements, err := Statements()
	if err != nil {
		t.Fatalf("load Cassandra schema: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected keyspace and table statements, got %d", len(statements))
	}
	if !strings.Contains(statements[0], "CREATE KEYSPACE") {
		t.Fatalf("expected keyspace statement first, got %q", statements[0])
	}
	if !strings.Contains(statements[1], "PRIMARY KEY ((conversation_key, bucket), message_seq)") {
		t.Fatalf("expected bucketed timeline primary key, got %q", statements[1])
	}
}
