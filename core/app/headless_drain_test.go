// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// headless_drain_test.go — end-to-end gate: DrainSync populates list rows
// without the manual ApplyResourcesLoaded seam.
package app_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newDemoController builds a Controller backed by demo/fake clients so that
// Core.ExecuteTask can call the registered paginated fetcher (e.g. EC2)
// without real AWS credentials. core.SetIsDemo(true) skips Wave-2 enrichment
// probes that require a non-demo session.
func newDemoController() (*runtime.Core, *app.Controller) {
	s := session.New()
	s.Profile = demo.DemoProfile
	s.Region = demo.DemoRegion
	// Wire fake clients so the catalog fetcher receives a non-nil *ServiceClients.
	// Without this, ec2/s3/rds fetchers return "AWS clients not initialized".
	s.Clients = demo.NewServiceClients()
	s.NoCache = true

	core := runtime.New(s, nil)
	core.SetIsDemo(true)

	return core, app.New(core)
}

// TestHeadless_FetchPopulatesListRows drives the controller the same way the
// web host does — Apply(ActionCommand), DrainSync, Snapshot — through the real
// task-result lane, never the ApplyResourcesLoaded seam.
func TestHeadless_FetchPopulatesListRows(t *testing.T) {
	tests := []struct {
		resourceType    string
		wantIDSubstring string // substring expected in at least one row's ResourceID
	}{
		{
			resourceType:    "ec2",
			wantIDSubstring: "i-0a1b2c3d4e5f6", // prefix of fixture instance IDs
		},
		{
			resourceType:    "s3",
			wantIDSubstring: "a9s-demo", // prefix of fixture bucket names
		},
	}

	for _, tc := range tests {
		t.Run(tc.resourceType, func(t *testing.T) {
			_, c := newDemoController()

			vs, tasks := c.Apply(app.Action{Kind: app.ActionCommand, Arg: tc.resourceType})

			if vs.Body.Kind != app.BodyKindList {
				t.Fatalf("Apply(%q): expected BodyKindList, got %q — controller did not push a list screen", tc.resourceType, vs.Body.Kind)
			}
			if len(tasks) == 0 {
				t.Fatalf("Apply(%q): returned 0 tasks — no fetch task was enqueued; check HandleNavigate task generation", tc.resourceType)
			}

			app.DrainSync(c, tasks)

			snap := c.Snapshot()
			if snap.Body.List == nil {
				t.Fatalf("%s: Snapshot().Body.List is nil after DrainSync — controller lost the list screen", tc.resourceType)
			}
			rows := snap.Body.List.Rows
			if len(rows) == 0 {
				t.Fatalf("%s: Body.List.Rows is empty after DrainSync — ResourcesLoaded event did not reach applyResourcesLoaded. "+
					"Possible breaks: (a) ExecuteTask returned error with nil clients, "+
					"(b) Handle did not route messages.ResourcesLoaded, "+
					"(c) handleResourcesLoadedEvent type-mismatch on screen stack", tc.resourceType)
			}

			found := false
			for _, row := range rows {
				if strings.Contains(row.ResourceID, tc.wantIDSubstring) {
					found = true
					break
				}
			}
			if !found {
				ids := make([]string, 0, len(rows))
				for _, row := range rows {
					ids = append(ids, row.ResourceID)
				}
				t.Fatalf("%s: no row has ResourceID containing %q — fixture data did not reach list rows. Got %d rows with IDs: %v",
					tc.resourceType, tc.wantIDSubstring, len(rows), ids)
			}
		})
	}
}

// TestHeadless_RowCellsArePopulated confirms that cells in the list rows are
// non-empty strings (not just scaffolded empty slices), proving the column
// extraction path (buildListBody / resolveListColumns) ran end-to-end.
func TestHeadless_RowCellsArePopulated(t *testing.T) {
	tests := []struct {
		resourceType string
		wantMinCells int // minimum expected columns per row
	}{
		{resourceType: "ec2", wantMinCells: 3},
		{resourceType: "s3", wantMinCells: 1},
	}

	for _, tc := range tests {
		t.Run(tc.resourceType, func(t *testing.T) {
			_, c := newDemoController()

			_, tasks := c.Apply(app.Action{Kind: app.ActionCommand, Arg: tc.resourceType})
			app.DrainSync(c, tasks)

			snap := c.Snapshot()
			if snap.Body.List == nil || len(snap.Body.List.Rows) == 0 {
				t.Fatalf("%s: no rows after DrainSync — prerequisite for cell check not met", tc.resourceType)
			}

			firstRow := snap.Body.List.Rows[0]
			if len(firstRow.Cells) < tc.wantMinCells {
				t.Fatalf("%s: first row has %d cells, want >= %d", tc.resourceType, len(firstRow.Cells), tc.wantMinCells)
			}
			hasNonEmpty := false
			for _, cell := range firstRow.Cells {
				if cell != "" {
					hasNonEmpty = true
					break
				}
			}
			if !hasNonEmpty {
				t.Fatalf("%s: first row has %d cells but all are empty — buildListBody did not extract field values", tc.resourceType, len(firstRow.Cells))
			}
		})
	}
}
