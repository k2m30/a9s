package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteJSON_FailedWriteLeavesPreviousFileIntact pins that a snapshot
// write that cannot complete leaves the previous snapshot.json byte-identical.
// An in-place truncate destroys every other type's section on the way to
// failing, so a later "--types x" refresh would carry nothing.
//
// The read-only directory is the deterministic stand-in for the interrupted
// write: creating a sibling temp file is refused, while opening the existing
// file for truncation is not.
func TestWriteJSON_FailedWriteLeavesPreviousFileIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not refuse the write")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	original := []byte(`{
  "region": "eu-west-2",
  "collected_at": "2026-01-01T00:00:00Z",
  "types": {
    "ebs": {"volumes": [{"id": "vol-0a1b2c3d4e5f60001"}]},
    "s3": {"buckets": [{"name": "acme-prod-assets"}]}
  }
}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("writing the pre-existing snapshot: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("making the snapshot directory read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700) //nolint:errcheck // restoring the mode so t.TempDir can clean up
	})

	err := writeJSON(path, snapshotFile{
		Region:      "eu-west-2",
		CollectedAt: "2026-01-02T00:00:00Z",
		Types:       map[string]any{"ec2": map[string]any{"instances": []any{}}},
	})
	if err == nil {
		t.Error("writeJSON reported success on a directory it cannot write into")
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading the snapshot back: %v", readErr)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("a failed write changed the previous snapshot.json.\ngot:\n%s\nwant:\n%s", got, original)
	}
}

// TestWriteJSON_SuccessLeavesOnlyTheTargetFile pins the successful half: the
// target holds the new content at owner-only mode and no temp file is left
// beside it for the next run to mistake for a snapshot.
func TestWriteJSON_SuccessLeavesOnlyTheTargetFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	if err := os.WriteFile(path, []byte(`{"types":{"ebs":{}}}`), 0o644); err != nil { //nolint:gosec // the wide mode is the condition under test
		t.Fatalf("writing the pre-existing snapshot: %v", err)
	}

	snap := snapshotFile{
		Region:      "eu-west-2",
		CollectedAt: "2026-01-02T00:00:00Z",
		Types: map[string]any{
			"ebs": json.RawMessage(`{"volumes":[]}`),
			"s3":  map[string]any{"buckets": []any{map[string]any{"name": "acme-prod-assets"}}},
		},
	}
	if err := writeJSON(path, snap); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing the snapshot directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "snapshot.json" {
		t.Errorf("snapshot directory holds %v, want only [snapshot.json]", names)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat snapshot.json: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("snapshot.json mode = %v, want %v", info.Mode().Perm(), os.FileMode(0o600))
	}

	var back struct {
		Types map[string]json.RawMessage `json:"types"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading snapshot.json: %v", err)
	}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("snapshot.json is not valid JSON after the write: %v\n%s", err, data)
	}
	for _, want := range []string{"ebs", "s3"} {
		if _, ok := back.Types[want]; !ok {
			t.Errorf("section %q missing from the written snapshot: %s", want, data)
		}
	}
}
