package main

import (
	"path/filepath"
	"testing"
)

// TestSnapshotDir_RelativeOutResolves pins that a relative --out still names a
// directory. Containment compares the pair directory against the root, and
// filepath.Join drops a leading "./", so "--out ." would otherwise be refused
// as a root that cannot contain anything.
func TestSnapshotDir_RelativeOutResolves(t *testing.T) {
	for _, out := range []string{".", "snap", "./snap", "tests/e2e/testdata/snapshot"} {
		got, err := snapshotDir(out, "example-readonly", "eu-west-2")
		if err != nil {
			t.Errorf("snapshotDir(%q, …) returned %v, want a resolved directory", out, err)
			continue
		}
		wantRoot, err := filepath.Abs(out)
		if err != nil {
			t.Fatalf("resolving %q: %v", out, err)
		}
		if want := filepath.Join(wantRoot, "example-readonly--eu-west-2"); got != want {
			t.Errorf("snapshotDir(%q, …) = %q, want %q", out, got, want)
		}
	}
}
