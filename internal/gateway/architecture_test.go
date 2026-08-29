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
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	paths := []string{
		filepath.Join(repositoryRoot, "cmd", "services", "gateway", "main.go"),
		filepath.Join(repositoryRoot, "internal", "services", "gateway", "bootstrap", "runtime.go"),
		filepath.Join(repositoryRoot, "internal", "gateway", "server.go"),
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
	}
}
