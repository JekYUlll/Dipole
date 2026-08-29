package bootstrap_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMessageRPCBootstrapUsesPlatformTransport(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Message bootstrap architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "rpc.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Message rpc bootstrap: %v", err)
	}
	if strings.Contains(string(source), "/internal/bootstrap") {
		t.Fatalf("Message production RPC bootstrap must not depend on legacy bootstrap: %s", path)
	}
	if !strings.Contains(string(source), "internal/platform/rpc") {
		t.Fatalf("Message production RPC bootstrap must use platform RPC transport: %s", path)
	}
}
