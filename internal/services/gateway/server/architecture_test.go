package gateway_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestGatewayRuntimeHasNoDatabaseOwnership(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve gateway architecture test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	paths := []string{
		filepath.Join(repositoryRoot, "cmd", "services", "gateway", "main.go"),
		filepath.Join(repositoryRoot, "internal", "services", "gateway", "bootstrap", "entrypoint.go"),
		filepath.Join(repositoryRoot, "internal", "services", "gateway", "bootstrap", "runtime.go"),
		filepath.Join(repositoryRoot, "internal", "services", "gateway", "server", "server.go"),
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", path, err)
			}
			if importPath == "database/sql" || strings.Contains(importPath, "/internal/data/") || strings.HasSuffix(importPath, "/internal/app") {
				t.Errorf("%s imports database ownership package %s", path, importPath)
			}
		}
		for _, forbidden := range []string{"InitMySQL(", "NewRepositories(", "NewMessageProcessRepositories("} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s contains forbidden database composition %s", path, forbidden)
			}
		}
		if strings.HasSuffix(path, filepath.Join("bootstrap", "entrypoint.go")) &&
			!strings.Contains(string(source), "return RunGatewayServer(server, tlsCfg)") {
			t.Errorf("%s must delegate RunServer to the Gateway-owned runtime", path)
		}
	}
}

func TestGatewayOwnsSyncHTTPHandler(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve gateway architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "server.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(source)
	if !strings.Contains(text, "syncHandler := NewSyncHandler(dependencies.Sync)") {
		t.Fatalf("Gateway must compose its service-owned Sync handler: %s", path)
	}
	if strings.Contains(text, "httpHandler.NewSyncHandler") {
		t.Fatalf("Gateway must not reintroduce the shared Sync handler: %s", path)
	}
}

func TestGatewayOwnsMessageHTTPHandler(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve gateway architecture test path")
	}
	path := filepath.Join(filepath.Dir(currentFile), "server.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(source)
	if !strings.Contains(text, "messageHandler := NewMessageHandler(dependencies.Messages)") {
		t.Fatalf("Gateway must compose its service-owned Message handler: %s", path)
	}
	if strings.Contains(text, "httpHandler.NewMessageHandler") {
		t.Fatalf("Gateway must not reintroduce the shared Message handler: %s", path)
	}
}

func TestGatewayServerDoesNotImportSharedHTTPHandlers(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve gateway architecture test path")
	}
	directory := filepath.Dir(currentFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read gateway server directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(source), "github.com/JekYUlll/Dipole/internal/gateway/http") {
			t.Fatalf("Gateway server must not import shared HTTP handlers: %s", path)
		}
	}
}
