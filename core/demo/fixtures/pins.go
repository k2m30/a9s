// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"fmt"
	"sort"
)

// Pin is one resource type's demo-visible arithmetic, declared beside the
// fixtures that produce it: the row count and the Wave-1 issue badge the main
// menu shows, and the state-coverage gaps that type is still allowed to have.
// It lives with the fixtures so a fixture change moves numbers in its own
// file instead of in a table every task shares.
type Pin struct {
	// ShortName is the resource type, e.g. "ec2".
	ShortName string
	// Rows is the number of rows the demo list holds.
	Rows int
	// Issues is the Wave-1 issue badge on the main menu — lower than the
	// settled badge for a type whose findings arrive in Wave 2.
	Issues int
	// Truncated is true when the page cap cuts this type's demo list, so the
	// menu renders both numbers with a "+" and Rows and Issues are lower
	// bounds rather than totals.
	Truncated bool
	// CoverageGaps are the "<bucket>" or "<finding code>" keys this type may
	// still miss a fixture witness for. The ratchet only ever shrinks it: a
	// gap that gains a witness fails until it is removed here.
	//
	// A bucket gap says no fixture of this type resolves to that
	// domain.Color, and every one of them is there for one of two reasons.
	// Either the classifier and the FindingDef table have no path to that
	// color at all — most "dim" entries: the Color func has no Dim branch and
	// no registered finding carries SevDim, so no fixture of any shape could
	// witness it. Or AWS itself cannot present the state beside healthy rows —
	// a torn-down resource stops appearing in the list API rather than
	// reporting a deleted status. A finding-code gap says no fixture produces
	// that documented finding yet, which is fixture debt and is expected to
	// burn down.
	CoverageGaps []string
}

var pins = map[string]Pin{}

// Register records one type's Pin. It is called from the type's own fixture
// file and panics on a duplicate, which can only be a copied declaration.
func Register(p Pin) {
	if _, dup := pins[p.ShortName]; dup {
		panic(fmt.Sprintf("fixtures: duplicate demo pin for %q", p.ShortName))
	}
	pins[p.ShortName] = p
}

// Pins returns every registered Pin, ordered by ShortName.
func Pins() []Pin {
	out := make([]Pin, 0, len(pins))
	for _, p := range pins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ShortName < out[j].ShortName })
	return out
}

// CoverageGaps returns every registered gap keyed "<shortName>:<gap>", the
// shape the demo-state coverage ratchet looks up.
func CoverageGaps() map[string]bool {
	out := make(map[string]bool)
	for _, p := range Pins() {
		for _, gap := range p.CoverageGaps {
			out[p.ShortName+":"+gap] = true
		}
	}
	return out
}
