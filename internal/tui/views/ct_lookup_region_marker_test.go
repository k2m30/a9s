// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// A list read from another Region says so whether or not it found anything.
// An empty answer from us-east-1 and an empty history in the session Region
// are the same screen otherwise, and telling them apart is what the marker is
// for — an operator who cannot tell reads a wrong-Region lookup as "no
// activity".
func TestRenderList_EmptyListNamesTheRegionItWasReadFrom(t *testing.T) {
	m := &ResourceListModel{}

	empty := app.ListBody{Columns: []app.ColumnDef{{Title: "EVENT", Width: 10}}, LookupRegion: "us-east-1"}
	if out := m.RenderList(empty); !strings.Contains(out, "us-east-1") {
		t.Errorf("an empty list read from us-east-1 renders %q, which does not say where it was read", out)
	}

	sessionRegion := app.ListBody{Columns: []app.ColumnDef{{Title: "EVENT", Width: 10}}}
	if out := m.RenderList(sessionRegion); strings.Contains(out, "── from") {
		t.Errorf("an empty list read from the session Region renders a Region marker: %q", out)
	}
}
