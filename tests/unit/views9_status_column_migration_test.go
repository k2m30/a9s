// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// A type whose status column is not the column titled "State".
//
// sfn_execution_history titles two columns after words that read as a status:
// "Status" carries the verdict its fetcher computes for the event, and "State"
// carries the name of the step in the operator's own state machine definition.
// Only the first is the status column, and a view file an older build wrote
// has to come out of the upgrade saying so.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// TestUpgradeKeepsTheStateNameColumnOnItsOwnKey drives a view file written
// before the status key moved and edited since, which is the one shape the
// wholesale column-set rule does not cover and the per-column migration does.
func TestUpgradeKeepsTheStateNameColumnOnItsOwnKey(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "sfn_execution_history", `generated: 6
list:
  Timestamp:
    path: Timestamp
    key: timestamp
    width: 30
  Event Type:
    key: event_type_short
    width: 24
  State:
    key: state_name
    width: 24
  Detail:
    key: event_detail
    width: 40

detail:
  - Timestamp
`)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Fatalf("LoadFromDirs: %v", err)
	}

	// The file's own keys, not the resolved ones: the render cascade merges
	// the type's key onto every built-in title on every frame, so a wrong key
	// in the file shows up only when the operator reads it, or renames the
	// column and puts it beyond the merge.
	want := map[string]string{"State": "state_name", "Status": "status"}
	for _, col := range cfg.Views["sfn_execution_history"].List {
		w, ok := want[col.Title]
		if !ok {
			continue
		}
		if col.Key != w {
			t.Errorf("the migrated file gives the %s column key %q, want %q", col.Title, col.Key, w)
		}
		delete(want, col.Title)
	}
	for title := range want {
		t.Errorf("the migrated file has no %s column", title)
	}
}
