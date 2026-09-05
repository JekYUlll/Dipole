package ai

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestAgentRuntimeDependsOnCapabilityPortInsteadOfRepositories(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"context_builder.go", "tools.go"} {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(payload)
		if !strings.Contains(source, "application.AgentCapabilityV1") {
			t.Errorf("%s does not depend on AgentCapabilityV1", path)
		}
		for _, forbidden := range []string{"type userReader interface", "type messageReader interface", "type conversationReader interface", "type systemMessageSender interface"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s reintroduced repository-shaped Agent dependency %q", path, forbidden)
			}
		}
	}
}

func TestAgentReplyDependsOnCommandPortInsteadOfMessageService(t *testing.T) {
	t.Parallel()

	payload, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	source := string(payload)
	if !strings.Contains(source, "application.AgentCommandV1") {
		t.Fatal("Agent reply does not depend on AgentCommandV1")
	}
	for _, forbidden := range []string{"MessageService", "LocalMessageApplication", "SendAssistantTextMessage"} {
		if strings.Contains(source, forbidden) {
			t.Errorf("Agent reply reintroduced direct Message dependency %q", forbidden)
		}
	}
}

func TestLegacyAgentIsLimitedToEmbeddedKafkaRollback(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	const legacyImport = "github.com/JekYUlll/Dipole/internal/services/agent/legacy"
	allowedImporters := map[string]struct{}{
		"internal/services/core/bootstrap/agentchat/direct_reply.go": {},
	}
	found := make([]string, 0, 1)

	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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
			if importPath == legacyImport {
				relativePath, _ := filepath.Rel(repositoryRoot, path)
				found = append(found, filepath.ToSlash(relativePath))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}
	if len(found) != len(allowedImporters) {
		t.Fatalf("legacy Agent importers = %v, want only agentchat", found)
	}
	for _, path := range found {
		if _, ok := allowedImporters[path]; !ok {
			t.Fatalf("legacy Agent importers = %v, unexpected %s", found, path)
		}
	}
}
