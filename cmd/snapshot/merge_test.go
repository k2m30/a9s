package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeSnapshotFile_PreservesNumericPrecision pins the fix for the
// finding: a partial run must carry over untouched type sections as raw
// bytes, not through json.Unmarshal into `any` (which downcasts every
// number to float64 and silently rounds anything above 2^53).
func TestMergeSnapshotFile_PreservesNumericPrecision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	const bigInt = "9007199254740993" // 2^53 + 1 — first int64 float64 cannot represent exactly

	existing := `{
  "region": "eu-west-2",
  "collected_at": "2026-01-01T00:00:00Z",
  "types": {
    "ebs": {"volumes": [{"size_bytes": ` + bigInt + `}]},
    "s3": {"buckets": [{"name": "old-bucket"}]}
  },
  "errors": {}
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write fixture snapshot.json: %v", err)
	}

	// Partial run selects "s3" only — "ebs" (holding the big int) must be
	// carried over untouched.
	types, _ := mergeSnapshotFile(path, []string{"s3"})

	rawEBS, ok := types["ebs"]
	if !ok {
		t.Fatalf("expected carried-over %q section, got types=%v", "ebs", types)
	}
	if !strings.Contains(string(rawEBS), bigInt) {
		t.Errorf("carried-over ebs section lost numeric precision: got %s, want literal digits %s present", rawEBS, bigInt)
	}

	// Simulate the save step: build a new snapshotFile from the merged
	// sections plus a freshly captured "s3" section, marshal it, and check
	// the big int survives on disk byte-for-byte — the operator-visible
	// truth the finding is about.
	newSnap := snapshotFile{
		Region:      "eu-west-2",
		CollectedAt: "2026-01-02T00:00:00Z",
		Types:       map[string]any{},
		Errors:      map[string]string{},
	}
	for k, v := range types {
		newSnap.Types[k] = v
	}
	newSnap.Types["s3"] = map[string]any{"buckets": []any{map[string]any{"name": "new-bucket"}}}

	outPath := filepath.Join(dir, "out.json")
	if err := writeJSON(outPath, newSnap); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read written snapshot: %v", err)
	}
	if !strings.Contains(string(out), bigInt) {
		t.Errorf("saved snapshot.json corrupted numeric precision: want literal digits %s present, got:\n%s", bigInt, out)
	}
}

// TestMergeSnapshotFile_SelectedSectionsNotCarried pins that types passed in
// `selected` are excluded from the merge result — they will be replaced by a
// freshly captured value by the caller, and a stale carried-over copy would
// silently mask a capture failure.
func TestMergeSnapshotFile_SelectedSectionsNotCarried(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	existing := `{
  "region": "eu-west-2",
  "collected_at": "2026-01-01T00:00:00Z",
  "types": {
    "ebs": {"volumes": []},
    "s3": {"buckets": []},
    "ec2": {"instances": []}
  },
  "errors": {}
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write fixture snapshot.json: %v", err)
	}

	types, _ := mergeSnapshotFile(path, []string{"s3", "ec2"})

	if _, ok := types["s3"]; ok {
		t.Errorf("selected type %q must not be carried over, got %v", "s3", types)
	}
	if _, ok := types["ec2"]; ok {
		t.Errorf("selected type %q must not be carried over, got %v", "ec2", types)
	}
	if _, ok := types["ebs"]; !ok {
		t.Errorf("unselected type %q must be carried over, got %v", "ebs", types)
	}
	if len(types) != 1 {
		t.Errorf("expected exactly 1 carried-over type, got %d: %v", len(types), types)
	}
}

// TestMergeSnapshotFile_MissingFile pins that a missing/absent snapshot.json
// (e.g. first-ever partial run, or a typo'd --out) is not an error — the
// caller gets empty maps and proceeds with a full effective capture for the
// selected types.
func TestMergeSnapshotFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	types, errs := mergeSnapshotFile(path, []string{"s3"})

	if len(types) != 0 {
		t.Errorf("expected empty types map for missing file, got %v", types)
	}
	if len(errs) != 0 {
		t.Errorf("expected empty errors map for missing file, got %v", errs)
	}
}

// TestMergeSnapshotFile_ErrorsCarriedExceptSelected pins that prior capture
// errors for unselected types survive a partial run (so the operator still
// sees which sections are stale/broken), while errors for selected types are
// dropped since they are about to be re-attempted.
func TestMergeSnapshotFile_ErrorsCarriedExceptSelected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	existing := `{
  "region": "eu-west-2",
  "collected_at": "2026-01-01T00:00:00Z",
  "types": {
    "ebs": {"volumes": []}
  },
  "errors": {
    "s3": "AccessDenied: no permission",
    "ec2": "timeout after 30s"
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write fixture snapshot.json: %v", err)
	}

	_, errs := mergeSnapshotFile(path, []string{"ec2"})

	if got, ok := errs["s3"]; !ok || got != "AccessDenied: no permission" {
		t.Errorf("expected unselected type %q error carried over verbatim, got %q (ok=%v)", "s3", got, ok)
	}
	if _, ok := errs["ec2"]; ok {
		t.Errorf("selected type %q error must be dropped (about to be retried), got %v", "ec2", errs)
	}
	if len(errs) != 1 {
		t.Errorf("expected exactly 1 carried-over error, got %d: %v", len(errs), errs)
	}
}

// TestMergeSnapshotFile_MalformedFile pins that a present-but-corrupt
// snapshot.json (e.g. truncated by a killed process) degrades to empty maps
// rather than propagating a decode error into the caller's control flow —
// consistent with the original inline `if err := json.Unmarshal(...); err ==
// nil` guard that silently ignored decode failures.
func TestMergeSnapshotFile_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(path, []byte(`{"types": {`), 0o600); err != nil {
		t.Fatalf("failed to write fixture snapshot.json: %v", err)
	}

	types, errs := mergeSnapshotFile(path, []string{"s3"})

	if len(types) != 0 {
		t.Errorf("expected empty types map for malformed file, got %v", types)
	}
	if len(errs) != 0 {
		t.Errorf("expected empty errors map for malformed file, got %v", errs)
	}
}

// compileTimeContractCheck pins the exact function signature the coder must
// implement — this line alone is the "red": it will not compile until
// mergeSnapshotFile is extracted from main() with this exact contract.
var _ func(string, []string) (map[string]json.RawMessage, map[string]string) = mergeSnapshotFile
