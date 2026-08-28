package main

import "testing"

func TestRunRequiresExplicitDryRunOutput(t *testing.T) {
	if err := run("job", "owner", "", "", 100, 60, false, "", "", "", ""); err == nil {
		t.Fatal("expected dry-run output requirement")
	}
}

func TestRunRequiresApprovalInputsForExecute(t *testing.T) {
	if err := run("job", "owner", "", "", 100, 60, true, "", "", "", ""); err == nil {
		t.Fatal("expected execute approval requirement")
	}
}
