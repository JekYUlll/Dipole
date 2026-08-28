package delivery

import (
	"strings"
	"testing"
)

func TestDecodeStrictJSONRejectsNestedDuplicateFields(t *testing.T) {
	type nested struct {
		Nodes []FenceExpectedNode `json:"nodes"`
	}
	if _, err := DecodeStrictJSON[nested]([]byte(`{"nodes":[{"component":"gateway","component":"cpp","observer_id":"node-a"}]}`)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("DecodeStrictJSON() error = %v", err)
	}
}
