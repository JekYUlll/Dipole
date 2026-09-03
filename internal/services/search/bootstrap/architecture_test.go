package bootstrap_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSearchRPCBootstrapUsesPlatformTransport(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Search bootstrap architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "rpc.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Search rpc bootstrap: %v", err)
	}
	if strings.Contains(string(source), "/internal/bootstrap") {
		t.Fatalf("Search production RPC bootstrap must not depend on legacy bootstrap: %s", path)
	}
	if !strings.Contains(string(source), "internal/platform/rpc") {
		t.Fatalf("Search production RPC bootstrap must use platform RPC transport: %s", path)
	}
}

func TestSearchRuntimesDoNotReintroduceMySQLIndexing(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Search bootstrap architecture test path")
	}
	directory := filepath.Dir(currentFile)
	for _, filename := range []string{"runtime.go", filepath.Join("..", "..", "search-indexer", "bootstrap", "runtime.go")} {
		path := filepath.Join(directory, filename)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Search runtime source %s: %v", path, err)
		}
		if strings.Contains(string(source), "platform/mysql") || strings.Contains(string(source), "infrastructure/mysql") {
			t.Fatalf("Search runtime must use Elasticsearch projection only: %s", path)
		}
	}
}
