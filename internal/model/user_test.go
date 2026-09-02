package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUserStatusContractMatchesDomainConstants(t *testing.T) {
	t.Parallel()

	payload, err := os.ReadFile(filepath.Join("..", "..", "contracts", "user", "v1", "status.schema.json"))
	if err != nil {
		t.Fatalf("read User status contract: %v", err)
	}
	var contract struct {
		ID      string          `json:"$id"`
		Version string          `json:"x-dipole-version"`
		Default int8            `json:"default"`
		Enum    []int8          `json:"enum"`
		Symbols map[string]int8 `json:"x-dipole-symbols"`
	}
	if err := json.Unmarshal(payload, &contract); err != nil {
		t.Fatalf("decode User status contract: %v", err)
	}

	if contract.ID != "https://dipole.local/contracts/user/v1/status.schema.json" {
		t.Fatalf("unexpected User status contract ID %q", contract.ID)
	}
	if contract.Version != "dipole.user.status.v1" {
		t.Fatalf("unexpected User status contract version %q", contract.Version)
	}
	if contract.Default != UserStatusNormal {
		t.Fatalf("contract default = %d, want %d", contract.Default, UserStatusNormal)
	}
	want := map[string]int8{
		"normal":   UserStatusNormal,
		"disabled": UserStatusDisabled,
	}
	if len(contract.Enum) != len(want) || len(contract.Symbols) != len(want) {
		t.Fatalf("unexpected User status contract cardinality: enum=%v symbols=%v", contract.Enum, contract.Symbols)
	}
	seen := make(map[int8]bool, len(contract.Enum))
	for _, value := range contract.Enum {
		seen[value] = true
	}
	for name, value := range want {
		if contract.Symbols[name] != value || !seen[value] {
			t.Fatalf("User status %s=%d is missing from contract: enum=%v symbols=%v", name, value, contract.Enum, contract.Symbols)
		}
	}
}

func TestUserStatusValuesAreStable(t *testing.T) {
	t.Parallel()

	if UserStatusNormal != 1 || UserStatusDisabled != 2 {
		t.Fatalf("User status values drifted: normal=%d disabled=%d", UserStatusNormal, UserStatusDisabled)
	}
}
