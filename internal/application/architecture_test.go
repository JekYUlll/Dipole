package application_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestTransportPackagesDoNotImportDataImplementations(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	for _, relativeDirectory := range []string{"internal/services/core/server", "internal/services/gateway/server", "internal/gateway/http", "internal/transport"} {
		directory := filepath.Join(repositoryRoot, relativeDirectory)
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if strings.Contains(importPath, "/internal/data/") || strings.HasSuffix(importPath, "/internal/store") {
					relativePath, _ := filepath.Rel(repositoryRoot, path)
					t.Errorf("%s imports data implementation %s", relativePath, importPath)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", relativeDirectory, err)
		}
	}
}

func TestApplicationContractsDoNotImportServiceImplementations(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	directory := filepath.Join(repositoryRoot, "internal", "application")
	forbiddenFragments := []string{
		"/internal/services/",
		"/internal/store",
		"/internal/data/",
		"/internal/operations/",
	}
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			forbidden := false
			for _, fragment := range forbiddenFragments {
				if strings.Contains(importPath, fragment) {
					forbidden = true
					break
				}
			}
			if forbidden {
				relativePath, _ := filepath.Rel(repositoryRoot, path)
				t.Errorf("%s imports service implementation %s", relativePath, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan application contracts: %v", err)
	}
}
