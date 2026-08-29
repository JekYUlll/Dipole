package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStrictJSONRejectsUnknownAndTrailingData(t *testing.T) {
	for _, payload := range []string{
		`{"name":"valid","extra":true}`,
		`{"name":"valid"}{"name":"second"}`,
		`{"name":"first","name":"second"}`,
	} {
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readStrictJSON[struct {
			Name string `json:"name"`
		}](path); err == nil {
			t.Fatal("invalid JSON input must fail")
		}
	}
}
