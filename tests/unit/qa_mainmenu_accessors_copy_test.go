package unit

// qa_mainmenu_accessors_copy_test.go — Regression: GetIssueCounts/GetIssueTruncated/
// GetIssueKnown return independent copies.
//
// Bug: Accessors were returning references to internal maps — callers mutating
// the returned map would corrupt the menu's internal state.
// Fix: Each accessor creates and returns a fresh copy via maps.Copy.
//
// Tests fail if the fix is reverted (i.e., the accessor returns the internal
// map directly): mutating the returned value would corrupt the next call.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestGetIssueCounts_ReturnsCopy verifies that mutating the returned map does
// not affect the menu's internal issueCounts.
func TestGetIssueCounts_ReturnsCopy(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 24)
	m.SetIssues("ec2", 5, false)

	c := m.GetIssueCounts()
	c["ec2"] = 999 // mutate the copy

	got := m.GetIssueCounts()["ec2"]
	if got != 5 {
		t.Fatalf("GetIssueCounts returned a reference, not a copy: mutating the return value changed the internal count from 5 to %d", got)
	}
}

// TestGetIssueTruncated_ReturnsCopy verifies that mutating the returned map does
// not affect the menu's internal issueTruncated.
func TestGetIssueTruncated_ReturnsCopy(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 24)
	m.SetIssues("rds", 3, true) //nolint:gocritic // comment documents the argument value, not commented-out code

	c := m.GetIssueTruncated()
	c["rds"] = false // flip the copy

	got := m.GetIssueTruncated()["rds"]
	if !got {
		t.Fatal("GetIssueTruncated returned a reference, not a copy: mutating the return value changed the internal truncated flag from true to false")
	}
}

// TestGetIssueKnown_ReturnsCopy verifies that mutating the returned map does
// not affect the menu's internal issueKnown.
func TestGetIssueKnown_ReturnsCopy(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 24)

	// SetIssues populates the known map implicitly.
	m.SetIssues("s3", 0, false)

	c := m.GetIssueKnown()
	c["s3"] = false // corrupt the copy

	got := m.GetIssueKnown()["s3"]
	if !got {
		t.Fatal("GetIssueKnown returned a reference, not a copy: mutating the return value changed the internal known flag from true to false")
	}
}
