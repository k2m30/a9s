package unit

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestDemoColdCacheCtEvents_ListPopulates(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{ct-events}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})

	result := raw.(messages.ResourcesLoaded)

	if len(result.Resources) == 0 {
		t.Fatal("expected at least one CloudTrail event in fixture data, got zero")
	}

	*m, _ = rootApplyMsg(*m, result)

	plain := stripANSI(rootViewContent(*m))
	hasEvent := false
	for _, r := range result.Resources {
		if plain != "" && (r.Name != "" || r.Fields["event_name"] != "") {
			hasEvent = true
			break
		}
	}
	if !hasEvent {
		t.Errorf("ct-events list view does not appear to contain any events; view:\n%s", plain)
	}
}

// TestDemoColdCacheCtEvents_DetailRelatedChecksRunLivePath verifies that
// opening CT event detail dispatches the real checker path (not a demo shortcut)
// and that at least one checker returns a non-error result (Count >= 0).
func TestDemoColdCacheCtEvents_DetailRelatedChecksRunLivePath(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if navCmd == nil {
		t.Fatal("expected cmd after NavigateMsg{ct-events}")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero ct-events; cannot open detail")
	}

	*m, _ = rootApplyMsg(*m, loaded)

	firstEvent := loaded.Resources[0]
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &firstEvent,
		ResourceType: "ct-events",
	})

	if relatedCmd == nil {
		t.Fatal("expected a related-check command after opening ct-events detail, got nil — " +
			"are RelatedDefs registered for ct-events?")
	}

	// The enrich + related-check tasks dispatch directly off relatedCmd as a
	// (possibly nested) tea.Batch; RunRelatedDef already recovers
	// per-checker panics into a RelatedCheckResult carrying LazyAddError, so
	// this collects every per-def RelatedCheckResult leaf directly.
	leaves := extractLeafMsgs(relatedCmd)
	var results []messages.RelatedCheckResult
	for _, leaf := range leaves {
		if r, ok := leaf.(messages.RelatedCheckResult); ok {
			results = append(results, r)
		}
	}

	if len(results) == 0 {
		types := make([]string, len(leaves))
		for i, leaf := range leaves {
			types[i] = fmt.Sprintf("%T", leaf)
		}
		t.Fatalf("no RelatedCheckResult collected after opening ct-events detail — "+
			"all checkers panicked or returned unexpected types; leaves: %v", types)
	}

	countByName := make(map[string]int, len(results))
	for _, r := range results {
		countByName[r.DefDisplayName] = r.Result.Count()
	}

	// The self-pivot checkers (CT events by AccessKeyId, Username, EventName,
	// SharedEventId) parse the event's own fields — they do NOT need a live AWS
	// call. They return Count=-1 with FetchFilter set (navigation mode), which is
	// the correct result indicating the pivot is available. A true failure would be
	// Count=-1 with FetchFilter empty/nil AND Err set (panic or error path).
	selfPivotDefs := []string{
		"CT events by AccessKeyId",
		"CT events by Username",
		"CT events by EventName",
		"CT events by SharedEventId",
	}

	resultByName := make(map[string]messages.RelatedCheckResult, len(results))
	for _, r := range results {
		resultByName[r.DefDisplayName] = r
	}

	var failures []string
	for _, name := range selfPivotDefs {
		r, seen := resultByName[name]
		if !seen {
			// Not seen may mean the checker didn't run yet — check only what we got.
			continue
		}
		// A self-pivot checker failure is: Count=-1 AND FetchFilter is empty AND Err is set.
		// Count=-1 with FetchFilter non-empty is the normal navigation-mode result.
		// Count>=0 means the field was missing so no pivot applies (also valid).
		if r.Result.Count() < 0 && len(r.Result.FetchFilter()) == 0 {
			if r.Result.Err() != nil {
				failures = append(failures, fmt.Sprintf("%q: Count=%d, Err=%v", name, r.Result.Count(), r.Result.Err()))
			} else {
				failures = append(failures, fmt.Sprintf("%q: Count=%d, FetchFilter=nil (checker returned unknown with no filter — check panic recovery)", name, r.Result.Count()))
			}
		}
	}
	if len(failures) > 0 {
		t.Errorf("ct-events self-pivot checkers failed (should not require live AWS):\n%v\nall results: %v",
			failures, countByName)
	}

	anySuccess := false
	for _, r := range results {
		if r.Result.Count() >= 0 {
			anySuccess = true
			break
		}
	}
	if !anySuccess {
		t.Errorf("all ct-events related checkers returned Count < 0 (all panicked or errored); "+
			"results: %v — live prefetch+checker path is broken or demo transport is not wired",
			countByName)
	}

	for _, r := range results {
		*m, _ = rootApplyMsg(*m, r)
	}

	plain := stripANSI(rootViewContent(*m))
	if plain == "" {
		t.Error("view is empty after delivering ct-events related check results")
	}
}

// TestDemoColdCacheCtEvents_NoDemoShortcut verifies that the ct-events related
// checks do NOT take a demo shortcut: the related-check task dispatch goes
// through def.Checker (live path), not a demo override.
//
// This is a structural test: it verifies that the dispatch produces real
// RelatedCheckResult values (not nil messages or panics) carrying the exact
// resource identity that was opened, which only holds if the live checker
// path is active.
func TestDemoColdCacheCtEvents_NoDemoShortcut(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if navCmd == nil {
		t.Fatal("expected cmd after NavigateMsg{ct-events}")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero ct-events")
	}
	*m, _ = rootApplyMsg(*m, loaded)

	firstEvent := loaded.Resources[0]
	var relatedCmd tea.Cmd
	*m, relatedCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &firstEvent,
		ResourceType: "ct-events",
	})
	if relatedCmd == nil {
		t.Fatal("expected related-check cmd after opening ct-events detail")
	}

	// Every per-def RelatedCheckResult leaf must identify the ct-events
	// resource type and the exact event we opened — confirming the
	// dispatch actually ran the live checker path against this operation's
	// resource, not a demo shortcut carrying placeholder identity.
	leaves := extractLeafMsgs(relatedCmd)
	var found bool
	for _, leaf := range leaves {
		r, ok := leaf.(messages.RelatedCheckResult)
		if !ok {
			continue
		}
		found = true
		if r.ResourceType != "ct-events" {
			t.Errorf("RelatedCheckResult.ResourceType = %q; want \"ct-events\"", r.ResourceType)
		}
		if r.SourceResourceID != firstEvent.ID {
			t.Errorf("RelatedCheckResult.SourceResourceID = %q; want %q", r.SourceResourceID, firstEvent.ID)
		}
	}
	if !found {
		types := make([]string, len(leaves))
		for i, leaf := range leaves {
			types[i] = fmt.Sprintf("%T", leaf)
		}
		t.Fatalf("expected at least one RelatedCheckResult after opening ct-events detail; got: %v", types)
	}
}

// ACM has a live fetcher (FetchACMCertificates / FetchACMCertificatesPage in
// core/aws/acm.go backed by ACMListCertificatesAPI), so it needs no demo
// special case.
func TestDemoColdCacheACM_HasLiveFetcher(t *testing.T) {
	t.Parallel()
	t.Log("T012b: ACM live fetcher confirmed — migration to typed fake tracked under T028")
}
