// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_failed_page_routing_test.go — a failure is the other outcome of the
// same request.
//
// Every fetch the terminal issues has two endings: a page of rows, or the
// error that came instead. The success carries the screen that asked and the
// sequence it asked at, so it lands on that screen and a stale one is
// discarded. The failure has to carry the same two facts or it is routed by
// type alone — onto whichever screen of that type is topmost — and can never
// be recognised as stale.
package unit_test

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// failedPageModel is a sized root model with an ec2 list screen open and no
// AWS clients, so every fetch it issues comes back as messages.APIError
// through the real adapter.
func failedPageModel(t *testing.T) tui.Model {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	m := newBlessedModel(t, "example-readonly", "us-east-1")
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	m = sized.(tui.Model)
	opened, _ := m.Update(messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	return opened.(tui.Model)
}

// TestFailedPage_CarriesTheScreenAndSequenceItsSuccessWouldHave pins the
// first of the two fetch commands. The failure must name the screen that
// asked and the sequence it asked at, exactly as the success does.
func TestFailedPage_CarriesTheScreenAndSequenceItsSuccessWouldHave(t *testing.T) {
	m := failedPageModel(t)

	msg := m.FetchResourcesCmdForTest("ec2", m.Core().AvailabilityGen())()
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("a fetch with no clients returned %T, want messages.APIError — this pin has no failure to route", msg)
	}
	if apiErr.ScreenID == 0 {
		t.Errorf("the failed fetch names no screen (ScreenID=0) while its paired success names one, so it " +
			"is applied to whichever list of this type is topmost rather than the one that asked")
	}
	if apiErr.ListSeq == 0 {
		t.Errorf("the failed fetch carries no sequence (ListSeq=0) while its paired success carries one, so " +
			"a failure superseded by a later refresh can never be recognised as stale")
	}
}

// TestFailedLoadMore_CarriesTheScreenAndSequenceItsSuccessWouldHave pins the
// second command. A load-more's failure retires an activity flag on a
// specific screen, so it needs the same two facts as the fetch above.
func TestFailedLoadMore_CarriesTheScreenAndSequenceItsSuccessWouldHave(t *testing.T) {
	m := failedPageModel(t)

	_, cmd := m.Update(messages.LoadMore{
		ResourceType:      "ec2",
		ContinuationToken: "page-2",
		Provenance:        messages.FetchProvenanceCanonicalList,
	})
	if cmd == nil {
		t.Fatal("LoadMore produced no command, so this pin has no load-more to fail")
	}
	apiErr, ok := cmd().(messages.APIError)
	if !ok {
		t.Fatalf("a load-more with no clients returned %T, want messages.APIError", cmd())
	}
	if apiErr.ScreenID == 0 {
		t.Errorf("the failed load-more names no screen (ScreenID=0) while its paired success names one")
	}
	if apiErr.ListSeq == 0 {
		t.Errorf("the failed load-more carries no sequence (ListSeq=0) while its paired success carries one")
	}
}

// TestFailedPage_ReachesTheDrillThatAskedAndAStaleOneIsDropped is the
// consequence the two fields exist for, driven through the controller a
// stamped failure reaches. A drill sits on its parent; the drill's own fetch
// fails, and the parent must not be the screen that shows the error. A
// failure the drill has already superseded shows nothing at all.
func TestFailedPage_ReachesTheDrillThatAskedAndAStaleOneIsDropped(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	c.PushChildListScreen("ec2")
	c.PatchListEscPops(true)
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0ddd444444444444d", "drill")}, nil, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	stale := guardFetchTask(t, tasks)
	_, tasks2 := c.Apply(app.Action{Kind: app.ActionRefresh})
	live := guardFetchTask(t, tasks2)

	c.Handle(messages.APIError{
		ResourceType: "ec2",
		Err:          errFailedPage,
		Provenance:   messages.FetchProvenanceChild,
		ScreenID:     stale.ScreenID,
		ListSeq:      stale.ListSeq,
	})
	if got := listFetchErrorOf(t, c); got != "" {
		t.Errorf("the drill shows %q after a failure it had already superseded — a stale failure marks a "+
			"screen whose newer request is still out", got)
	}

	c.Handle(messages.APIError{
		ResourceType: "ec2",
		Err:          errFailedPage,
		Provenance:   messages.FetchProvenanceChild,
		ScreenID:     live.ScreenID,
		ListSeq:      live.ListSeq,
	})
	if got := listFetchErrorOf(t, c); got == "" {
		t.Error("the drill shows no error after its own fetch failed")
	}

	// The parent underneath never asked for this page and must be clean.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if got := listFetchErrorOf(t, c); got != "" {
		t.Errorf("the list under the drill shows %q — the drill's failure was applied to a screen that did "+
			"not issue it", got)
	}
}

// listFetchErrorOf is the error marker the top list screen renders.
func listFetchErrorOf(t *testing.T, c *app.Controller) string {
	t.Helper()
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body on the snapshot")
	}
	return body.LastFetchError
}

var errFailedPage = errors.New("operation error EC2: DescribeInstances, api error RequestLimitExceeded")
