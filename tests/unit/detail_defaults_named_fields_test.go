// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// A fact an operator can only read off the detail view has to be declared
// there: nothing else on the rendered surface carries it.
func TestDetailDefaultsDeclareOperatorOnlyFields(t *testing.T) {
	cases := []struct {
		shortName string
		path      string
		why       string
	}{
		{
			"dbi-snap", "SourceDBSnapshotIdentifier",
			"a copy of another snapshot names its parent here; without it the row shows a " +
				"DB Instances (0) beside an identifier and nothing says it is a copy",
		},
		{
			"mwaa", "LoggingConfiguration",
			"whether task, scheduler, worker and webserver logging is on is a detail-view fact " +
				"for mwaa and is raised as no finding",
		},
	}

	for _, tc := range cases {
		t.Run(tc.shortName+"/"+tc.path, func(t *testing.T) {
			vd := config.DefaultViewDef(tc.shortName)
			declared := make([]string, 0, len(vd.Detail))
			for _, f := range vd.Detail {
				declared = append(declared, f.String())
			}
			if !slices.Contains(declared, tc.path) {
				t.Errorf("%s detail does not declare %q — %s\ndeclared: %v",
					tc.shortName, tc.path, tc.why, declared)
			}
		})
	}
}
