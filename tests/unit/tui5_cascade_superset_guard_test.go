// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_cascade_superset_guard_test.go — the pin that replaces the
// first-column-title guard on the superset arm of ResolveListColumnCascade.
//
// The guard existed to stop a catalog type whose own layout differs from the
// built-in view from being switched to the built-in defaults wholesale. It
// never fired: across every registered type the two declarations agree about
// the first column, so the arm it guarded was taken every time it was
// reachable. The column-agreement gate (cols_one_resolver_test.go and the
// per-type column pins) is what guards the cascade now; this test pins the
// premise the guard's deletion rests on, so a future type that DOES disagree
// about its first column shows up here instead of silently changing layout.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestCascadeSupersetArm_EveryTypeAgreesOnItsFirstColumn walks every
// registered resource type, and for each one whose built-in view declares
// more columns than the catalog literal does (the superset arm's condition),
// asserts the two declarations name the same first column.
func TestCascadeSupersetArm_EveryTypeAgreesOnItsFirstColumn(t *testing.T) {
	var disagreed []string
	var reached int

	for _, td := range resource.AllResourceTypes() {
		defaultVD := config.GetViewDef(nil, td.ShortName)
		if len(defaultVD.List) <= len(td.Columns) {
			continue
		}
		reached++
		if len(td.Columns) == 0 {
			continue
		}
		if defaultVD.List[0].Title != td.Columns[0].Title {
			disagreed = append(disagreed,
				td.ShortName+": defaults say "+defaultVD.List[0].Title+
					", catalog says "+td.Columns[0].Title)
		}
	}

	if reached == 0 {
		t.Fatal("no registered type reaches the superset arm — the probe proves nothing")
	}
	if len(disagreed) > 0 {
		t.Errorf("%d type(s) reach the superset arm with a different first column, so the arm changes their layout:\n  %s",
			len(disagreed), strings.Join(disagreed, "\n  "))
	}
	t.Logf("superset arm reachable for %d of %d registered types; all agree on the first column",
		reached, len(resource.AllResourceTypes()))
}
