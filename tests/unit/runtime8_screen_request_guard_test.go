// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_screen_request_guard_test.go — a list screen guards its own
// requests.
//
// Two list screens of one type are open at once whenever a drill sits on top
// of the list it was opened from. The ordering guard that decides which of two
// results for a list is the newer one is keyed by the type, so it cannot tell
// those two screens apart: one screen's request moves the type's sequence on,
// and the other screen's answers are ordered against a number that has nothing
// to do with it. Every list screen already carries a stable instance identity
// (the ScreenID a dispatch stamps on its own tasks), which is the identity the
// guard has to key by.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// guardRow is one EC2 row whose Name says which fetch produced it, so a
// rendered cell names the answer that won.
func guardRow(id, name string) resource.Resource {
	return resource.Resource{
		ID: id, Name: name, Type: "ec2",
		Fields: map[string]string{"instance_id": id, "name": name, "state": "running"},
	}
}

// guardFetchTask picks the resource fetch out of what an action dispatched.
func guardFetchTask(t *testing.T, tasks []runtime.TaskRequest) runtime.TaskRequest {
	t.Helper()
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			return task
		}
	}
	t.Fatalf("the action dispatched no resource fetch: %v", tasks)
	return runtime.TaskRequest{}
}

// guardListNames returns the Name cell of every row the top list renders.
func guardListNames(t *testing.T, c *app.Controller) []string {
	t.Helper()
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body on the snapshot")
	}
	var names []string
	for _, row := range body.Rows {
		names = append(names, row.ResourceID)
	}
	return names
}

// guardOpenDrill pushes a second ec2 list screen on top of the canonical one:
// a related drill, which is the same type and a different screen.
func guardOpenDrill(t *testing.T, c *app.Controller, rows []resource.Resource) {
	t.Helper()
	c.PushChildListScreen("ec2")
	c.PatchListEscPops(true)
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
}

// TestListRequestGuard_ADrillOrdersItsOwnRefreshes pins the shape: a drill's
// second refresh answers after its first, and the drill renders what the
// second one returned. Today the sequence is drawn per canonical type and a
// drill draws none at all, so neither of its own results is ordered against
// the other and whichever lands last wins — the operator watches a refreshed
// list revert to the rows the refresh replaced.
func TestListRequestGuard_ADrillOrdersItsOwnRefreshes(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	guardOpenDrill(t, c, []resource.Resource{guardRow("i-0ddd444444444444d", "drill-v0")})

	_, tasks1 := c.Apply(app.Action{Kind: app.ActionRefresh})
	first := guardFetchTask(t, tasks1)
	_, tasks2 := c.Apply(app.Action{Kind: app.ActionRefresh})
	second := guardFetchTask(t, tasks2)

	if first.ScreenID == 0 || first.ScreenID != second.ScreenID {
		t.Fatalf("the drill's two refreshes carry screen ids %d and %d — both were issued by the same "+
			"screen and must name it", first.ScreenID, second.ScreenID)
	}
	if first.ListSeq == second.ListSeq {
		t.Errorf("the drill's two refreshes were both stamped sequence %d, so neither is ordered against "+
			"the other and the older answer overwrites the newer one", first.ListSeq)
	}

	newRow := guardRow("i-0eee555555555555e", "drill-v2")
	oldRow := guardRow("i-0fff666666666666f", "drill-v1")
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2", Resources: []resource.Resource{newRow},
		Provenance: messages.FetchProvenanceChild,
		ScreenID:   second.ScreenID, ListSeq: second.ListSeq,
	})
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2", Resources: []resource.Resource{oldRow},
		Provenance: messages.FetchProvenanceChild,
		ScreenID:   first.ScreenID, ListSeq: first.ListSeq,
	})

	got := guardListNames(t, c)
	if len(got) != 1 || got[0] != newRow.ID {
		t.Errorf("after the drill's newer refresh landed and its older one arrived late, the drill renders "+
			"%v, want just %q — a superseded answer replaced the one that superseded it", got, newRow.ID)
	}
}

// TestListRequestGuard_TheCanonicalListStillOrdersItsOwnRefreshes is the
// healthy counterpart, and it passes today: the canonical list is the one
// screen the per-type sequence happens to be right for. Re-keying the guard by
// the screen instance must keep it right — a drill that starts guarding its
// own requests at the cost of the parent losing the ordering it has is not the
// fix.
func TestListRequestGuard_TheCanonicalListStillOrdersItsOwnRefreshes(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "v0")}, nil, false)

	_, tasks1 := c.Apply(app.Action{Kind: app.ActionRefresh})
	first := guardFetchTask(t, tasks1)
	_, tasks2 := c.Apply(app.Action{Kind: app.ActionRefresh})
	second := guardFetchTask(t, tasks2)

	if first.ListSeq == second.ListSeq {
		t.Fatalf("the canonical list's two refreshes were both stamped sequence %d", first.ListSeq)
	}

	newRow := guardRow("i-0eee555555555555e", "v2")
	oldRow := guardRow("i-0fff666666666666f", "v1")
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2", Resources: []resource.Resource{newRow},
		Provenance: messages.FetchProvenanceCanonicalList,
		ScreenID:   second.ScreenID, ListSeq: second.ListSeq,
	})
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2", Resources: []resource.Resource{oldRow},
		Provenance: messages.FetchProvenanceCanonicalList,
		ScreenID:   first.ScreenID, ListSeq: first.ListSeq,
	})

	got := guardListNames(t, c)
	if len(got) != 1 || got[0] != newRow.ID {
		t.Errorf("the canonical ec2 list renders %v after its older refresh arrived late, want just %q",
			got, newRow.ID)
	}
}
