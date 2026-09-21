// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Every type declares its status column.
//
// The cascade reads the status cell from the column's own key and from
// nothing else. That makes a mistyped key a blank cell rather
// than a value arriving from another spelling, so the declaration itself is
// what has to be pinned: exactly one column carries the type's lifecycle key,
// and no column that reads as the status column to an operator is left out of
// it.
package unit

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestEveryTypeDeclaresOneStatusColumn sweeps the whole catalog, parents and
// children. A type must not declare two status columns, and where it has a
// column an operator reads as the status column — one titled "Status" or
// "State" — one of those titles must be the declared one.
//
// The title a word carries is the type's to decide: sfn_execution_history's
// "State" is the name of a step in the operator's state machine definition,
// and its verdict lives in the "Status" column beside it. What an operator
// cannot be left with is a table where every title that reads as a status
// names some other key, because then no cell on the row ever shows a finding
// phrase.
// looksLikeLifecycleKey reports whether a key names a lifecycle verdict rather
// than something that merely reads like one. sfn_execution_history's "State"
// column over "state_name" is the name of a step in the operator's state
// machine, not a verdict, and a raw AWS enum stored under "state",
// "cluster_status" or the like is.
func looksLikeLifecycleKey(key string) bool {
	return key == "state" || key == "status" ||
		strings.HasSuffix(key, "_state") || strings.HasSuffix(key, "_status")
}

func TestEveryTypeDeclaresOneStatusColumn(t *testing.T) {
	types := append(resource.AllResourceTypes(), resource.AllChildTypes()...)
	for _, td := range types {
		lifecycleKey := td.LifecycleKey
		if lifecycleKey == "" {
			lifecycleKey = "state"
		}

		declared, titled := 0, 0
		titledIsDeclared := false
		for _, col := range td.Columns {
			isStatus := config.IsStatusColumn(col.Key, lifecycleKey)
			if isStatus {
				declared++
			}
			if col.Title == "Status" || col.Title == "State" {
				titled++
				titledIsDeclared = titledIsDeclared || isStatus
				if !isStatus && looksLikeLifecycleKey(col.Key) {
					t.Errorf("%s titles a column %q over the key %q, beside the declared lifecycle key %q — "+
						"two cells an operator reads as the verdict, and only one of them ever carries a "+
						"finding phrase",
						td.ShortName, col.Title, col.Key, lifecycleKey)
				}
			}
		}

		if declared > 1 {
			t.Errorf("%s declares %d status columns — the lifecycle key %q names one column, and a "+
				"second one carrying it makes the cell depend on which is found first",
				td.ShortName, declared, lifecycleKey)
		}
		if titled > 0 && !titledIsDeclared {
			t.Errorf("%s has a column titled Status or State and none of them names the type's "+
				"lifecycle key %q — the cell reads the key the column declares, so each shows "+
				"whatever the fetcher happens to store there and never a finding phrase",
				td.ShortName, lifecycleKey)
		}
	}
}
