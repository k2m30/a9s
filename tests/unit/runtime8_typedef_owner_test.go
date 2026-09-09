// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_typedef_owner_test.go — one typeDef owner per screen, in the
// findings/colour lane.
//
// A screen's typeDef is what its columns, its title and its Wave-2 fold are
// resolved from. The controller owns one resolver for it (typeDefForLocked),
// which walks the catalog AND the child-type registry. A site in the same lane
// that asks the catalog directly gets nil for every child list, so the screen
// renders from a typeDef the rest of the screen is not using.
package unit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ownerChildType registers a child type for the duration of the test. Child
// types live outside the catalog, which is the whole difference between the
// screen's owner and a bare catalog lookup.
func ownerChildType(t *testing.T, shortName, listTitle string) resource.ResourceTypeDef {
	t.Helper()
	def := resource.ResourceTypeDef{
		Name:      listTitle,
		ShortName: shortName,
		ListTitle: listTitle,
		Columns: []domain.Column{
			{Key: "key", Title: "Key", Width: 40},
			{Key: "status", Title: "Status", Width: 20},
		},
	}
	resource.SetChildTypeForTest(def)
	t.Cleanup(func() { resource.CleanupChildTypeForTest(shortName) })
	return def
}

// TestTypeDefOwner_ChildListTitleComesFromTheScreensOwner pins the title. The
// child type declares a ListTitle; the screen showing it must use it, exactly
// as a catalog type's list does. Resolved through the catalog alone it is nil
// and the frame falls back to the raw short name, so the operator reads an
// internal identifier where every other list reads a title.
func TestTypeDefOwner_ChildListTitleComesFromTheScreensOwner(t *testing.T) {
	def := ownerChildType(t, "runtime8_child_objects", "S3 Objects")

	c := newTestController(t)
	c.PushChildListScreen(def.ShortName)
	c.ApplyResourcesLoaded(def.ShortName, []resource.Resource{
		{ID: "a.log", Name: "a.log", Fields: map[string]string{"key": "a.log", "status": "ok"}},
	}, nil, false)

	title := c.Snapshot().FrameTitle
	if !strings.Contains(title, def.ListTitle) {
		t.Errorf("the child list's frame title is %q, want it to carry %q — the title was resolved "+
			"from the catalog alone, which holds no child type, while the columns beside it came "+
			"from the screen's own resolver", title, def.ListTitle)
	}
	if strings.Contains(title, def.ShortName) {
		t.Errorf("the child list's frame title is %q — it shows the internal short name because the "+
			"catalog lookup returned nothing", title)
	}
}

// TestTypeDefOwner_ChildRowsFoldWave2ThroughTheScreensOwner pins the colour
// lane. A Wave-2 finding on a child row has to reach that row: the fold is
// handed the screen's typeDef, and a catalog-only lookup hands it an empty one
// for every child list.
func TestTypeDefOwner_ChildRowsFoldWave2ThroughTheScreensOwner(t *testing.T) {
	def := ownerChildType(t, "runtime8_child_events", "Log Events")

	c, core := newTestControllerAndCore(t)
	c.PushChildListScreen(def.ShortName)
	rows := []resource.Resource{
		{ID: "evt-1", Name: "evt-1", Type: def.ShortName, Fields: map[string]string{"key": "evt-1", "status": "ok"}},
		{ID: "evt-2", Name: "evt-2", Type: def.ShortName, Fields: map[string]string{"key": "evt-2", "status": "ok"}},
	}
	c.ApplyResourcesLoaded(def.ShortName, rows, nil, false)
	core.ObserveRows(def.ShortName, rows, nil, session.OriginFetch, false)

	healthy := ""
	for _, row := range c.Snapshot().Body.List.Rows {
		if row.ResourceID == "evt-2" {
			healthy = row.Color
		}
	}

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: def.ShortName,
		Findings: map[string][]domain.Finding{"evt-1": {{
			Code: "runtime8.child-broken", Phrase: "delivery failed",
			Severity: domain.SevBroken, Source: "wave2",
		}}},
	})
	c.ApplyIntents(intents)

	body := c.Snapshot().Body.List
	var flagged, unflagged string
	for _, row := range body.Rows {
		switch row.ResourceID {
		case "evt-1":
			flagged = row.Color
		case "evt-2":
			unflagged = row.Color
		}
	}
	if flagged == healthy {
		t.Errorf("the child row carrying a broken Wave-2 finding renders colour %q, the same as before "+
			"the finding landed — the fold ran against an empty typeDef because the catalog holds no "+
			"child type", flagged)
	}
	if unflagged != healthy {
		t.Errorf("the child row with no finding moved from %q to %q", healthy, unflagged)
	}
}

// ownerLaneFiles are the core/app files that render a list screen's rows,
// title, columns and colours. Every typeDef a screen is drawn from is resolved
// in one of them.
var ownerLaneFiles = []string{
	"core/app/list_body.go",
	"core/app/list_columns.go",
	"core/app/list_filter.go",
	"core/app/footer.go",
}

// TestTypeDefOwner_TheLaneAsksTheOwner is the source gate. A screen's typeDef
// and its canonical short name each have one resolver: typeDefForLocked and
// resource.CanonicalShortName. A bare resource.FindResourceType in this lane
// is a second answer to one of those two questions — either a typeDef that
// skips the child registry, or CanonicalShortName spelled out inline.
//
// list_columns.go declares the owner, so the calls inside it are the answer
// rather than a second copy of it.
func TestTypeDefOwner_TheLaneAsksTheOwner(t *testing.T) {
	call := regexp.MustCompile(`resource\.FindResourceType\(`)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var violations []string
	owners := 0
	for _, rel := range ownerLaneFiles {
		raw, rerr := os.ReadFile(filepath.Join(root, rel))
		if rerr != nil {
			t.Fatalf("read %s: %v", rel, rerr)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if !call.MatchString(line) || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if rel == "core/app/list_columns.go" {
				owners++
				continue
			}
			violations = append(violations, fmt.Sprintf("%s:%d %s", rel, i+1, strings.TrimSpace(line)))
		}
	}

	if owners == 0 {
		t.Fatal("core/app/list_columns.go resolves no typeDef — the gate is not reading the owner it thinks it is")
	}
	if len(violations) > 0 {
		t.Errorf("%d site(s) in the list-rendering lane resolve a typeDef or a canonical name themselves "+
			"instead of asking the screen's owner (typeDefForLocked) or resource.CanonicalShortName, so a "+
			"child list is drawn from a typeDef the rest of the screen is not using:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

var _ = runtime.ScreenChildList
