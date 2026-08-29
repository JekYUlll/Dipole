package bootstrap_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGatewayRPCBootstrapUsesPlatformTransport(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Gateway bootstrap architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "rpc.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Gateway rpc bootstrap: %v", err)
	}
	if strings.Contains(string(source), "/internal/bootstrap") {
		t.Fatalf("Gateway production RPC bootstrap must not depend on legacy bootstrap: %s", path)
	}
	if !strings.Contains(string(source), "internal/platform/rpc") {
		t.Fatalf("Gateway production RPC bootstrap must use platform RPC transport: %s", path)
	}
}

func TestGatewayRuntimeUsesServiceKafkaInfrastructure(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Gateway bootstrap architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "runtime.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Gateway runtime: %v", err)
	}
	if strings.Contains(string(source), "/internal/bootstrap") {
		t.Fatalf("Gateway runtime must not depend on legacy bootstrap: %s", path)
	}
	if !strings.Contains(string(source), "services/gateway/infrastructure/kafka") {
		t.Fatalf("Gateway runtime must use service Kafka infrastructure: %s", path)
	}
}
