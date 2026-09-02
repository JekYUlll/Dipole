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

func TestCoreAgentToolReceiptQueryUsesRemoteSender(t *testing.T) {
	t.Parallel()

	local := &lazyCoreMessageSender{}
	remote := &lazyCoreMessageSender{}
	if got := coreAgentToolReceiptQuery(local, remote); got != remote {
		t.Fatal("standalone Core Agent Tool audit must query the remote Message service")
	}
	if got := coreAgentToolReceiptQuery(local, nil); got != local {
		t.Fatal("embedded Core Agent Tool audit must retain the local Message query")
	}
}
