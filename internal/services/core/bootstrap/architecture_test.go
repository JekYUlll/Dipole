package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreRPCBootstrapOwnsPlatformTransport(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("rpc.go"))
	if err != nil {
		t.Fatalf("read Core RPC bootstrap: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "internal/bootstrap\"") {
		t.Fatalf("Core RPC bootstrap must not import legacy bootstrap directly")
	}
	if !strings.Contains(text, "internal/platform/rpc") {
		t.Fatalf("Core RPC bootstrap must use platform RPC transport")
	}
}

func TestCoreServiceEntrypointUsesOwnedRuntime(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("entrypoint.go"))
	if err != nil {
		t.Fatalf("read Core entrypoint: %v", err)
	}
	if !strings.Contains(string(source), "return InitializeCoreService(ctx)") {
		t.Fatalf("Core service entrypoint must initialize its owned runtime")
	}
	if strings.Contains(string(source), "legacybootstrap.RunServer") {
		t.Fatalf("Core service entrypoint must own its HTTP/TLS server startup")
	}
}
