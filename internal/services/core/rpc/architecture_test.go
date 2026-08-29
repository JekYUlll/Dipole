package rpc

import (
	"os"
	"strings"
	"testing"
)

func TestCoreRPCCompositionDoesNotDependOnLegacyBootstrap(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read Core RPC composition: %v", err)
	}
	if strings.Contains(string(source), "internal/bootstrap") {
		t.Fatal("Core RPC composition must not depend on legacy bootstrap")
	}
}
