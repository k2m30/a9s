// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_fetch_identity_test.go — every fetch says which screen asked.
//
// Four fetch commands answer with the same two message types, through one
// outcome value. Two of them fill in the screen that issued the request and
// the sequence that orders it; two leave both zero, so their pages are routed
// by resource type and lane alone. A filtered drill stacked on a filtered
// drill, or a child list on a child list, is two screens the router cannot
// tell apart: the deeper one's page lands on the newer one.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// fetchOutcomeFields are the two the router needs and the two this gate is
// about; the rest of the outcome differs per lane by design.
var fetchOutcomeFields = []string{"screen", "seq"}

// TestFetchIdentity_EveryFetchCommandNamesItsScreen reads the four outcome
// constructions in the terminal's fetch adapter. They are the one place a
// request's identity is attached, so a construction that leaves it out is a
// lane whose pages cannot be routed to the screen that asked.
func TestFetchIdentity_EveryFetchCommandNamesItsScreen(t *testing.T) {
	const rel = "internal/tui/fetch_adapter.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join("..", "..", rel), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	var missing []string
	built := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		name, ok := lit.Type.(*ast.Ident)
		if !ok || name.Name != "fetchOutcome" {
			return true
		}
		built++
		set := map[string]bool{}
		for _, elt := range lit.Elts {
			kv, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}
			if key, isID := kv.Key.(*ast.Ident); isID {
				set[key.Name] = true
			}
		}
		var absent []string
		for _, field := range fetchOutcomeFields {
			if !set[field] {
				absent = append(absent, field)
			}
		}
		if len(absent) > 0 {
			missing = append(missing, fmt.Sprintf("%s:%d leaves %s unset", rel, fset.Position(lit.Pos()).Line, strings.Join(absent, " and ")))
		}
		return true
	})

	if built < 4 {
		t.Fatalf("only %d fetch outcomes found in %s; the gate is not reading the tree it thinks it is", built, rel)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d of %d fetch commands answer without naming the screen that asked, so their pages are "+
			"routed by resource type and lane alone:\n  %s", len(missing), built, strings.Join(missing, "\n  "))
	}
}

// TestFetchIdentity_AChildFetchNamesItsScreen drives the child lane through
// the real adapter with no AWS clients, so the command answers with the
// failure half of the same outcome.
func TestFetchIdentity_AChildFetchNamesItsScreen(t *testing.T) {
	m := failedPageModel(t)

	_, cmd := m.Update(messages.LoadResources{
		ResourceType:  "ec2",
		ParentContext: map[string]string{"cluster": "example-cluster"},
	})
	if cmd == nil {
		t.Fatal("a child load produced no command")
	}
	msg := cmd()
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("a child fetch with no clients returned %T, want messages.APIError", msg)
	}
	if apiErr.ScreenID == 0 {
		t.Error("the child fetch names no screen, so its page is applied to whichever child list of this " +
			"type is topmost rather than the one that asked")
	}
	if apiErr.ListSeq == 0 {
		t.Error("the child fetch draws no sequence, so its own two requests cannot be ordered against " +
			"each other")
	}
}

// TestFetchIdentity_TwoStackedDrillsEachKeepTheirOwnPages is the consequence.
// Two filtered lists of one type are stacked; the deeper one's page must not
// appear on the newer one.
func TestFetchIdentity_TwoStackedDrillsEachKeepTheirOwnPages(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	c.PushChildListScreen("ec2")
	c.PatchListEscPops(true)
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0bbb222222222222b", "lower")}, nil, false)
	lower := c.GetListInstance()

	c.PushChildListScreen("ec2")
	c.PatchListEscPops(true)
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0ccc333333333333c", "upper")}, nil, false)
	upper := c.GetListInstance()

	if lower == upper || lower == 0 {
		t.Fatalf("the two stacked drills share screen identity %d, so nothing can tell them apart", lower)
	}

	// The lower drill's own page, arriving while the upper one is on top.
	lowerPage := guardRow("i-0ddd444444444444d", "lower-page-2")
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{lowerPage},
		Provenance:   messages.FetchProvenanceChild,
		ScreenID:     lower,
	})

	if got := guardListNames(t, c); len(got) != 1 || got[0] != "i-0ccc333333333333c" {
		t.Errorf("the drill on top renders %v after the drill BENEATH it received a page — a result is "+
			"routed to whichever screen of its type and lane is topmost", got)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if got := guardListNames(t, c); len(got) != 1 || got[0] != lowerPage.ID {
		t.Errorf("the lower drill renders %v after its own page arrived, want just %q", got, lowerPage.ID)
	}
}

// TestFetchIdentity_APageNamingNoScreenReachesNeitherOfTwo pins the fallback's
// deletion. With two screens of one type and one lane stacked, a result that
// names no screen belongs to neither: routing it by type and lane picks the
// topmost, which is a guess.
func TestFetchIdentity_APageNamingNoScreenReachesNeitherOfTwo(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	for _, id := range []string{"i-0bbb222222222222b", "i-0ccc333333333333c"} {
		c.PushChildListScreen("ec2")
		c.PatchListEscPops(true)
		c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow(id, "drill")}, nil, false)
	}

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{guardRow("i-0eee555555555555e", "unrouted")},
		Provenance:   messages.FetchProvenanceChild,
	})

	if got := guardListNames(t, c); len(got) != 1 || got[0] != "i-0ccc333333333333c" {
		t.Errorf("the drill on top renders %v after a page that names no screen arrived — the by-type "+
			"fallback guessed, and the guess is the screen that happened to be on top", got)
	}
}

// TestFetchIdentity_APageNamingNoScreenReachesTheOnlyListEither is row 17's
// gate. The by-type scan was narrowed to fire only when one screen answers to
// the result's type and lane, which reads as safe and is not: no production
// dispatch leaves the identity off, so the only messages that reach it are the
// ones tests build by hand. A production path that routes a page by its type
// is a path kept alive for the test suite, and it is the path that put the
// deeper drill's page on the newer one.
//
// One canonical list, one page naming no screen: the page belongs to no
// screen, and the list keeps what it had.
func TestFetchIdentity_APageNamingNoScreenReachesTheOnlyListEither(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "opened")}, nil, false)

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{guardRow("i-0eee555555555555e", "unrouted")},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if got := guardListNames(t, c); len(got) != 1 || got[0] != "i-0aaa111111111111a" {
		t.Errorf("the ec2 list renders %v after a page that names no screen arrived, want just the row it "+
			"already had — the by-type scan is still routing pages nothing dispatched", got)
	}
}
