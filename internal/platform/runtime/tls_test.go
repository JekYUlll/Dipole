package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateTLSFilesAcceptsExistingCertificateAndKey(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	key := filepath.Join(dir, "server.key")
	if err := os.WriteFile(cert, []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTLSFiles(config.TLS{CertFile: cert, KeyFile: key}); err != nil {
		t.Fatalf("validate TLS files: %v", err)
	}
}

func TestValidateTLSFilesRejectsMissingKey(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	if err := os.WriteFile(cert, []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTLSFiles(config.TLS{CertFile: cert, KeyFile: filepath.Join(dir, "missing.key")}); err == nil {
		t.Fatal("expected missing TLS key to be rejected")
	}
}
