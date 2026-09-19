// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// List rows carry no row decorator.
//
// A list row's colour is the worst finding over both waves, so a row carrying
// a finding is never green for a glyph to annotate: docs/attention-signals.md
// and docs/architecture.md both place the "!"/"~" glyph as the per-entry
// marker inside the detail-view Attention section, not on a list row.
package unit_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// rowDecoratorNames are the spellings a row-decorator plumbing would go by.
// CellDecorators — a per-column cell-value transform with live
// registrations — is a different mechanism.
var rowDecoratorNames = []string{"RowDecorator", "DecoratorNormal", ".Decorator"}

// TestNoRowDecoratorPlumbing walks every production file and asserts none of
// them declares or consumes a row decorator.
func TestNoRowDecoratorPlumbing(t *testing.T) {
	roots := []string{"../../core", "../../internal", "../../cmd"}
	var offenders []string

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".html") {
				return nil
			}
			if strings.HasSuffix(name, "_test.go") {
				return nil
			}
			rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			// cmd/checklist has a Decorator field of its own on its own JSON
			// report struct, unrelated to the list body's rows.
			if strings.HasPrefix(rel, "cmd/checklist/") {
				return nil
			}
			data, readErr := os.ReadFile(path) //nolint:gosec // path comes from this walk of the repo's own tree
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(data), "\n") {
				for _, spelling := range rowDecoratorNames {
					if strings.Contains(line, spelling) {
						offenders = append(offenders,
							rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("row-decorator plumbing is still here, with nothing producing a decorator:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
