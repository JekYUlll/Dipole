package bootstrap

import (
	"strings"
	"testing"
)

func TestValidateStandaloneCoreMode(t *testing.T) {
	t.Parallel()

	if err := validateStandaloneCoreMode("remote"); err != nil {
		t.Fatalf("remote Core mode should be supported: %v", err)
	}
	for _, mode := range []string{"", "embedded", "invalid"} {
		if err := validateStandaloneCoreMode(mode); err == nil || !strings.Contains(err.Error(), "gateway.mode=remote") {
			t.Fatalf("mode %q should be rejected with an actionable error, got %v", mode, err)
		}
	}
}
