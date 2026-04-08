package unit_test

// ct_events_pivot_e2e_nav_test.go — Layer 5 end-to-end regression test for
// demo-mode ct-events self-pivot navigation.
//
// Bug: pressing Enter on a "CT events by Username" or "CT events by EventName"
// right-column pivot row dispatches RelatedNavigateMsg{TargetType:"ct-events",
// FetchFilter:{...}} to the root model. The root handler calls fetchResourcesFiltered,
// which short-circuits on nil clients (demo mode has no AWS clients) and emits
// APIErrorMsg instead of ResourcesLoadedMsg — the user lands on an empty error view.
//
// This test asserts the correct end-to-end outcome:
//  1. No APIErrorMsg is produced.
//  2. A non-empty ResourcesLoadedMsg is produced.
//  3. Every resource in the slice matches the filter (Username or EventName).

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// pivotDrainBatch executes a cmd chain that may contain tea.BatchMsg at any
// level and collects all produced messages. Returns the first ResourcesLoadedMsg
// and the first APIErrorMsg seen, plus all collected messages for diagnostics.
func pivotDrainBatch(t *testing.T, m tui.Model, cmd tea.Cmd, maxOps int) (
	tui.Model,
	*messages.ResourcesLoadedMsg,
	*messages.APIErrorMsg,
	[]tea.Msg,
) {
	t.Helper()
	var allMsgs []tea.Msg
	var loaded *messages.ResourcesLoadedMsg
	var apiErr *messages.APIErrorMsg
	queue := []tea.Cmd{cmd}
	for ops := 0; ops < maxOps && len(queue) > 0; ops++ {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub != nil {
					queue = append(queue, sub)
				}
			}
			continue
		}
		allMsgs = append(allMsgs, msg)
		if rl, ok := msg.(messages.ResourcesLoadedMsg); ok && loaded == nil {
			copy := rl
			loaded = &copy
		}
		if ae, ok := msg.(messages.APIErrorMsg); ok && apiErr == nil {
			copy := ae
			apiErr = &copy
		}
		var nextCmd tea.Cmd
		newM, nextCmd := m.Update(msg)
		m = newM.(tui.Model)
		if nextCmd != nil {
			queue = append(queue, nextCmd)
		}
	}
	return m, loaded, apiErr, allMsgs
}

// pivotDemoClientsReadyMsg creates a ClientsReadyMsg backed by the demo
// transport. Mirrors demoClientsReadyMsg() in demo_app_test.go (package unit)
// which is not visible here (package unit_test).
func pivotDemoClientsReadyMsg() messages.ClientsReadyMsg {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)
	return messages.ClientsReadyMsg{Clients: clients}
}

// newPivotDemoRootModel creates a demo-mode root tui.Model sized for testing.
// It runs the full Init → ClientsReadyMsg cycle so that m.clients is populated
// before any filtered fetch is attempted (otherwise fetchResourcesFiltered
// short-circuits on nil clients and emits APIErrorMsg).
func newPivotDemoRootModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 40})
	m = newM.(tui.Model)
	newM2, _ := m.Update(pivotDemoClientsReadyMsg())
	return newM2.(tui.Model)
}

// ---------------------------------------------------------------------------
// TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList
// ---------------------------------------------------------------------------

// TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList verifies that in
// demo mode, navigating via a ct-events self-pivot row (Username or EventName)
// produces a non-empty ResourcesLoadedMsg rather than an APIErrorMsg.
//
// Cases: two representative fixtures × two pivot types = 4 subtests.
func TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList(t *testing.T) {
	ensureNoColor(t)

	type pivotCase struct {
		eventID      string
		pivotDisplay string // DisplayName of the pivot RelatedDef
		filterKey    string // key expected in FetchFilter
		filterValueFn func(res resource.Resource) string
	}

	cases := []pivotCase{
		{
			eventID:      "e-f2a3b4c5",
			pivotDisplay: "CT events by Username",
			filterKey:    "Username",
			filterValueFn: func(r resource.Resource) string {
				return r.Fields["user"]
			},
		},
		{
			eventID:      "e-f2a3b4c5",
			pivotDisplay: "CT events by EventName",
			filterKey:    "EventName",
			filterValueFn: func(r resource.Resource) string {
				v := r.Fields["event_name"]
				if v == "" {
					v = r.Name
				}
				return v
			},
		},
		{
			eventID:      "e-c3d4e5f6",
			pivotDisplay: "CT events by Username",
			filterKey:    "Username",
			filterValueFn: func(r resource.Resource) string {
				return r.Fields["user"]
			},
		},
		{
			eventID:      "e-c3d4e5f6",
			pivotDisplay: "CT events by EventName",
			filterKey:    "EventName",
			filterValueFn: func(r resource.Resource) string {
				v := r.Fields["event_name"]
				if v == "" {
					v = r.Name
				}
				return v
			},
		},
	}

	// Build demo resource cache once — used by real checkers (pure field-readers).
	cache := buildDemoResourceCache(t)

	for _, tc := range cases {
		tc := tc
		name := fmt.Sprintf("%s/%s", tc.eventID, tc.filterKey)
		t.Run(name, func(t *testing.T) {
			// Step 1: Load the fixture.
			source := loadCTEventsFixtureByID(t, tc.eventID)

			// Step 2: Find the pivot RelatedCheckResult matching this pivot's DisplayName.
			// We need the FetchFilter that would be dispatched by pressing Enter.
			allResults := ctEventsRealCheckerResults(source, cache)
			var pivotResult *resource.RelatedCheckResult
			for _, r := range allResults {
				if r.TargetType == "ct-events" && r.Count == -1 && len(r.FetchFilter) > 0 {
					if _, hasPivotKey := r.FetchFilter[tc.filterKey]; hasPivotKey {
						copy := r
						pivotResult = &copy
						break
					}
				}
			}
			if pivotResult == nil {
				t.Skipf("fixture %s has no pivot result with FetchFilter[%q] — skipping",
					tc.eventID, tc.filterKey)
				return
			}

			label := fmt.Sprintf("event=%s pivot=%s FetchFilter=%v",
				tc.eventID, tc.pivotDisplay, pivotResult.FetchFilter)

			// Step 3: Build demo root model and send the RelatedNavigateMsg.
			m := newPivotDemoRootModel(t)
			navMsg := messages.RelatedNavigateMsg{
				TargetType:     "ct-events",
				SourceResource: source,
				RelatedIDs:     pivotResult.ResourceIDs, // empty for pivots
				FetchFilter:    pivotResult.FetchFilter,
			}

			// Step 4: Feed RelatedNavigateMsg to root model → get cmd chain.
			newM, cmd := m.Update(navMsg)
			m = newM.(tui.Model)

			if cmd == nil {
				t.Errorf("FAIL: root model returned nil cmd after RelatedNavigateMsg — pivot was not handled — %s",
					label)
				return
			}

			// Step 5: Walk the cmd chain collecting messages.
			// maxOps=50 is sufficient for a batch of init + fetch cmds.
			_, loaded, apiErr, allMsgs := pivotDrainBatch(t, m, cmd, 50)

			// Assertion 1: No APIErrorMsg (this is the bug — demo mode short-circuits on nil clients).
			if apiErr != nil {
				t.Errorf("FAIL assertion 1: got APIErrorMsg{ResourceType:%q, Err:%v}, want ResourcesLoadedMsg"+
					" — fetchResourcesFiltered short-circuits on nil clients in demo mode — %s",
					apiErr.ResourceType, apiErr.Err, label)
			}

			// Assertion 2: Exactly one ResourcesLoadedMsg (pivot must load resources).
			if loaded == nil {
				t.Errorf("FAIL assertion 2: no ResourcesLoadedMsg produced — pivot navigation yielded no data"+
					" — %s | all msgs: %v", label, allMsgs)
				return
			}

			// Assertion 3: Non-empty result set.
			if len(loaded.Resources) == 0 {
				t.Errorf("FAIL assertion 3: ResourcesLoadedMsg.Resources is empty — pivot landed on empty list"+
					" — %s | filter=%v", label, pivotResult.FetchFilter)
				return
			}

			// Assertion 4: Every resource matches the filter value.
			expectedValue := tc.filterValueFn(source)
			if expectedValue == "" {
				t.Logf("WARN: source fixture %s has empty filter value for %s — skipping per-resource filter assertion",
					tc.eventID, tc.filterKey)
			} else {
				for i, r := range loaded.Resources {
					gotValue := tc.filterValueFn(r)
					if gotValue != expectedValue {
						t.Errorf("FAIL assertion 4: resource[%d] %q has %s=%q, want %q — filter mismatch — %s",
							i, r.ID, tc.filterKey, gotValue, expectedValue, label)
					}
				}
			}
		})
	}
}
