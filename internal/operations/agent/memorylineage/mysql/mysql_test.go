package mysql

import (
	"testing"
)

func TestDecodeMemoryLineageManifest(t *testing.T) {
	manifest, err := decodeMemoryLineageManifest([]byte(`{"selected":[{"id":"memory:MEM-1","representation":"compact"}]}`))
	if err != nil {
		t.Fatalf("decodeMemoryLineageManifest() error = %v", err)
	}
	if len(manifest.Selected) != 1 || manifest.Selected[0].ID != "memory:MEM-1" || manifest.Selected[0].Representation != "compact" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestDecodeMemoryLineageManifestRejectsMalformedJSON(t *testing.T) {
	if _, err := decodeMemoryLineageManifest([]byte(`{"selected":`)); err == nil {
		t.Fatal("expected malformed manifest rejection")
	}
}

func TestMemoryLineageBackfillConstructorsRequireStore(t *testing.T) {
	if _, err := NewMemoryLineageBackfillSource(nil); err == nil {
		t.Fatal("expected source store requirement")
	}
	if _, err := NewMemoryLineageBackfillTarget(nil); err == nil {
		t.Fatal("expected target store requirement")
	}
	if _, err := NewMemoryLineageBackfillCheckpointStore(nil); err == nil {
		t.Fatal("expected checkpoint store requirement")
	}
}
