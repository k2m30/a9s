// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_codex_build_test.go pins what the off-lock body build reads: a
// map it shares with the locked writer (row 40), and the typeDef it resolves
// twice (row 39).
package unit_test

import (
	"context"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// --- row 40: a map on both sides of the lock -------------------------------

// TestOffLockBuild_DoesNotShareTheRelatedIDSet pins row 40. The build detaches
// the list state by copying the struct, which copies the related-ID set by
// reference. A body built off the lock then iterates the same map a result
// landing under the lock inserts into, and Go ends the process on that.
//
// Under the race detector this is the report; without it, the hazard is a
// runtime "concurrent map read and map write" that no assertion can catch
// afterwards.
func TestOffLockBuild_DoesNotShareTheRelatedIDSet(t *testing.T) {
	td := wipfixObjTypeDef()
	c, _ := newTestControllerAndCore(t)
	c.PushChildListScreen(td.ShortName)
	c.RegisterFallbackTypeDef(td)

	seed := make([]resource.Resource, 0, 64)
	ids := make([]string, 0, 64)
	for i := range 64 {
		id := "obj-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		ids = append(ids, id)
		seed = append(seed, resource.Resource{ID: id, Name: id, Fields: map[string]string{}})
	}
	c.ApplyResourcesLoaded(td.ShortName, seed, nil, false)
	c.SeedRelatedExactRows(td.ShortName, ids)

	// A checker that claims every row it is shown, so each reapply inserts
	// into the same related-ID set the build is iterating.
	c.PatchListReapplyChecker(func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		var found []string
		for _, entry := range cache {
			for _, r := range entry.Resources {
				found = append(found, r.ID)
			}
		}
		return resource.KnownRelated(td.ShortName, found, false)
	}, resource.Resource{ID: "parent", Name: "parent"})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// Handle captures the build inputs under the lock, runs the build off
		// it, and installs the memo back under it — the off-lock window this
		// row is about.
		for i := range 200 {
			id := "page-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
			_, _ = handlePage(c, messages.ResourcesLoaded{
				ResourceType: td.ShortName,
				Resources:    []resource.Resource{{ID: id, Name: id, Fields: map[string]string{}}},
				Pagination:   &domain.PaginationMeta{IsTruncated: false},
				Append:       true,
				Provenance:   messages.FetchProvenanceChild,
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			id := "late-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
			c.ApplyReapplyCheckerAgainst([]resource.Resource{
				{ID: id, Name: id, Fields: map[string]string{}},
			})
		}
	}()
	wg.Wait()

	if body := c.Snapshot().Body.List; body == nil {
		t.Fatal("no list body after the concurrent build and reapply")
	}
}

// --- row 39: one typeDef owner per screen ----------------------------------

// TestChildTypeResolvesTheSameOnBothPaths is row 39's behavioural half. A
// child type is registered outside the catalog, and only one of the two
// resolvers walks that rung: the display side finds the child's typeDef, the
// filter side finds nothing, so a column that renders cannot be filtered on.
func TestChildTypeResolvesTheSameOnBothPaths(t *testing.T) {
	c := newTestController(t)
	child := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "wipfix_child_objects",
		Columns: []domain.Column{
			{Key: "key", Title: "Key", Width: 40},
			{Key: "size", Title: "Size", Width: 10},
		},
	}
	resource.SetChildTypeForTest(child)
	t.Cleanup(func() { resource.CleanupChildTypeForTest(child.ShortName) })

	c.PushChildListScreen(child.ShortName)
	c.ApplyResourcesLoaded(child.ShortName, []resource.Resource{
		{ID: "a.log", Name: "a.log", Fields: map[string]string{"key": "a.log", "size": "10"}},
		{ID: "b.log", Name: "b.log", Fields: map[string]string{"key": "b.log", "size": "20"}},
	}, nil, false)

	body := c.Snapshot().Body.List
	if body == nil || len(body.Columns) == 0 {
		t.Fatalf("precondition: the child screen rendered no columns (%v)", body)
	}

	// The filter reads the same columns the screen renders, so filtering on
	// one of them narrows the list.
	_, _ = c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "a.log"})
	after := c.Snapshot().Body.List
	if after == nil {
		t.Fatal("no list body after filtering")
	}
	if len(after.Rows) != 1 {
		t.Errorf("filtering a child list on a value its own column renders left %d rows, want 1 — "+
			"the filter resolved a different typeDef from the one that rendered the columns",
			len(after.Rows))
	}
}
