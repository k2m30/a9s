// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_status_declared_test.go — every type declares its status column.
//
// The cascade reads the status cell from the column's own key and from
// nothing else (w197 row 11). That makes a mistyped key a blank cell rather
// than a value arriving from another spelling, so the declaration itself is
// what has to be pinned: exactly one column carries the type's lifecycle key,
// and no column that reads as the status column to an operator is left out of
// it.
package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestEveryTypeDeclaresOneStatusColumn sweeps the whole catalog, parents and
// children. A type with a column an operator reads as the status column — one
// titled "Status" or "State" — must declare that column, and a type must not
// declare two.
func TestEveryTypeDeclaresOneStatusColumn(t *testing.T) {
	types := append(resource.AllResourceTypes(), resource.AllChildTypesForTest()...)
	for _, td := range types {
		lifecycleKey := td.LifecycleKey
		if lifecycleKey == "" {
			lifecycleKey = "state"
		}

		declared, titled := 0, 0
		for _, col := range td.Columns {
			if config.IsStatusColumn(col.Key, lifecycleKey) {
				declared++
			}
			if col.Title == "Status" || col.Title == "State" {
				titled++
			}
		}

		if declared > 1 {
			t.Errorf("%s declares %d status columns — the lifecycle key %q names one column, and a "+
				"second one carrying it makes the cell depend on which is found first",
				td.ShortName, declared, lifecycleKey)
		}
		if titled > declared {
			t.Errorf("%s has a column titled Status or State that does not name the type's lifecycle "+
				"key %q — the cell reads the key the column declares, so this one shows whatever the "+
				"fetcher happens to store there and never a finding phrase",
				td.ShortName, lifecycleKey)
		}
	}
}
