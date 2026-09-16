package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSnapshotDir_EncodesPathElements pins that the output directory for one
// profile+region pair is built from ESCAPED path elements, the same encoding
// the on-disk cache uses for its pair directories. A raw join lets a profile
// or region carrying a separator reach a directory of its own choosing, and
// lets two different pairs collide on one directory and read each other's
// snapshot.
func TestSnapshotDir_EncodesPathElements(t *testing.T) {
	const out = "/tmp/snap"

	cases := []struct {
		name    string
		profile string
		region  string
		want    string
	}{
		{
			name:    "ordinary pair passes through byte-identical",
			profile: "example-readonly",
			region:  "eu-west-2",
			want:    "/tmp/snap/example-readonly--eu-west-2",
		},
		{
			name:    "a separator in the profile is escaped, not traversed",
			profile: "team/a",
			region:  "eu-west-2",
			want:    "/tmp/snap/team%2Fa--eu-west-2",
		},
		{
			name:    "a traversal region stays one element under --out",
			profile: "example-readonly",
			region:  "../../../target",
			want:    "/tmp/snap/example-readonly--..%2F..%2F..%2Ftarget",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := snapshotDir(out, c.profile, c.region)
			if err != nil {
				t.Fatalf("snapshotDir(%q, %q, %q) returned error %v, want a resolved directory",
					out, c.profile, c.region, err)
			}
			if got != c.want {
				t.Errorf("snapshotDir(%q, %q, %q) = %q, want %q", out, c.profile, c.region, got, c.want)
			}
			if clean := filepath.Clean(got); !strings.HasPrefix(clean, filepath.Clean(out)+string(os.PathSeparator)) {
				t.Errorf("resolved directory %q escapes --out %q", clean, out)
			}
		})
	}
}

// TestSnapshotDir_PairsNeverCollide pins the injective half of the encoding:
// the "--" joiner between the two elements cannot be forged from a boundary
// hyphen, so ("team-", "eu-west-2") and ("team", "-eu-west-2") keep separate
// directories instead of one silently overwriting the other's snapshot.
func TestSnapshotDir_PairsNeverCollide(t *testing.T) {
	const out = "/tmp/snap"

	a, err := snapshotDir(out, "team-", "eu-west-2")
	if err != nil {
		t.Fatalf("snapshotDir for (%q, %q): %v", "team-", "eu-west-2", err)
	}
	b, err := snapshotDir(out, "team", "-eu-west-2")
	if err != nil {
		t.Fatalf("snapshotDir for (%q, %q): %v", "team", "-eu-west-2", err)
	}
	if a == b {
		t.Errorf("two distinct profile+region pairs resolved to the same directory %q", a)
	}
}

// TestSnapshotDir_RefusesUnresolvablePair pins that a pair the encoding
// cannot contain under --out is refused with an error rather than returning a
// path the caller would then create. An empty --out has no root to contain
// anything.
func TestSnapshotDir_RefusesUnresolvablePair(t *testing.T) {
	got, err := snapshotDir("", "example-readonly", "eu-west-2")
	if err == nil {
		t.Fatalf("snapshotDir with an empty --out returned %q and no error, want a refusal", got)
	}
	if got != "" {
		t.Errorf("refused pair returned directory %q, want %q", got, "")
	}
}
