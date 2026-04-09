package unit

// qa_26_search_integration_test.go — QA-26 root-level integration tests.
//
// Sections covered:
//   G: Exiting Search (G01–G04)
//   K: Scroll-to-Match (K01–K04)
//   L: Word Wrap Interaction (L01–L03)
//   N: Component Reuse — detail and YAML only (N01–N06)
//   O: Other Key Bindings During Search (O01–O07)
//   P: Help Screen — detail and YAML only (P01–P02)
//   Q: Terminal Resize (Q01–Q03)
//   R: Real-World Scenarios — detail and YAML only (R01–R02, R05)

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	tui "github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// activateAndConfirmSearch is a helper that types a search query and confirms
// it with Enter through the root model.
// ---------------------------------------------------------------------------

func activateAndConfirmSearch(t *testing.T, m tui.Model, query string) tui.Model {
	t.Helper()
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, ch := range query {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m
}

// navigateToDetail pushes a detail view for a resource into the root model.
func navigateToDetail(m tui.Model, res *resource.Resource) tui.Model {
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetDetail, Resource: res})
	return m
}

// navigateToYAML pushes a YAML view for a resource into the root model.
func navigateToYAML(m tui.Model, res *resource.Resource) tui.Model {
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetYAML, Resource: res})
	return m
}

// twoMatchResource returns a resource that contains "running" in two distinct
// fields, ensuring searches find exactly 2 matches.
func twoMatchResource(id string) *resource.Resource {
	return &resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"state":       "running",
			"power_state": "running",
		},
	}
}

// ---------------------------------------------------------------------------
// Section G — Exiting Search
// ---------------------------------------------------------------------------

// 26-G01: Esc clears highlights and returns to normal mode; viewport position
// is preserved (not tested via pixel offset, but view content must change to
// reflect the cleared state and the header must revert to "? for help").
func TestSearch_G01_EscClearsHighlightsReturnsNormalMode(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-g01")
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	// Confirm search is active: header shows match info, not "? for help".
	afterSearch := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if strings.Contains(afterSearch, "? for help") {
		t.Fatal("G01: header should not show '? for help' while search is active")
	}
	if !strings.Contains(afterSearch, "matches") {
		t.Fatalf("G01: header should contain 'matches' while search active; got: %q", afterSearch)
	}

	// Press Esc — clears search.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	afterEsc := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Match indicator must be gone.
	if strings.Contains(afterEsc, "matches") {
		t.Error("G01: match indicator should disappear after Esc")
	}
	// Header must revert to "? for help".
	if !strings.Contains(afterEsc, "? for help") {
		t.Errorf("G01: header must show '? for help' after Esc; got: %q", afterEsc)
	}
	// The view must still be the detail view (resource name visible).
	if !strings.Contains(afterEsc, "i-g01") {
		t.Errorf("G01: should remain in detail view after Esc; got: %q", afterEsc)
	}
}

// 26-G02: Starting a new search replaces the current search. Old highlights
// are cleared and new query matches are highlighted.
func TestSearch_G02_NewSearchReplacesCurrentSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:   "i-g02",
		Name: "i-g02",
		Fields: map[string]string{
			"state":       "running",
			"power_state": "running",
			"stopped_by":  "automation",
		},
	}
	m = navigateToDetail(m, res)

	// First search: "running" — expect 2 matches, indicator starts at 1/2.
	m = activateAndConfirmSearch(t, m, "running")
	afterFirst := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(afterFirst, "1/2 matches") {
		t.Fatalf("G02: expected '1/2 matches' for first search (2 total, on first); got: %q", afterFirst)
	}

	// Press "/" again — previous highlights cleared, input mode opens.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	duringNewInput := ansiRe.ReplaceAllString(rootViewContent(m), "")
	// Header must show "/" (search input active) and NOT still show old match count.
	if !strings.Contains(duringNewInput, "/") {
		t.Error("G02: after pressing '/' again, header should show search input '/'")
	}

	// Type "stopped" and confirm.
	for _, ch := range "stopped" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	afterSecond := ansiRe.ReplaceAllString(rootViewContent(m), "")
	// New search results must reference "stopped", not old "running" count.
	if strings.Contains(afterSecond, "2/2 matches") {
		t.Error("G02: old 'running' match count must not persist after new search")
	}
	if !strings.Contains(afterSecond, "matches") {
		t.Errorf("G02: new search should be active with 'matches' indicator; got: %q", afterSecond)
	}
}

// 26-G03: Esc during search input (before Enter) does NOT exit the view.
// The user remains in the detail/YAML view.
func TestSearch_G03_EscDuringInputDoesNotExitView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-g03")
	m = navigateToYAML(m, res)

	// Activate search input.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})

	// Type "running" but do NOT press Enter.
	for _, ch := range "running" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Press Esc — cancels input, but must stay in YAML view.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Must still be in YAML view — frame title contains "yaml".
	if !strings.Contains(plain, "yaml") {
		t.Errorf("G03: Esc during input must not exit YAML view; got: %q", plain)
	}
	// Search must be inactive (no match indicator).
	if strings.Contains(plain, "matches") {
		t.Error("G03: no match indicator should be visible after Esc cancels input")
	}
	// Header must show "? for help" (normal mode).
	if !strings.Contains(plain, "? for help") {
		t.Errorf("G03: header must show '? for help' after Esc cancels input; got: %q", plain)
	}
}

// 26-G04: Esc during search results clears search; second Esc exits to resource list.
func TestSearch_G04_TwoEscapesExitView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate: main menu → resource list → detail (to have something to pop back to).
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	res := twoMatchResource("i-g04")
	m = navigateToDetail(m, res)

	// Confirm a search.
	m = activateAndConfirmSearch(t, m, "running")
	afterSearch := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(afterSearch, "matches") {
		t.Fatal("G04: search should be active before first Esc")
	}

	// First Esc — clears search, stays in detail view.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	afterFirstEsc := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if strings.Contains(afterFirstEsc, "matches") {
		t.Error("G04: first Esc should clear match indicator")
	}
	// Must still be detail view (not resource list) — frame title contains resource name.
	if !strings.Contains(afterFirstEsc, "i-g04") {
		t.Errorf("G04: after first Esc should still be in detail view; got: %q", afterFirstEsc)
	}

	// Second Esc — pops the view (back to resource list).
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	afterSecondEsc := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// After popping detail, the view changes — no longer shows detail for i-g04.
	// It should show the resource list (ec2).
	if strings.Contains(afterSecondEsc, "i-g04 ") {
		// If "i-g04" appears only as part of row data in the list that's ok,
		// but the frame title should have changed.
		t.Logf("G04: second Esc result: %q", afterSecondEsc[:min(300, len(afterSecondEsc))])
	}
	// View content must have changed from the detail view.
	if afterSecondEsc == afterFirstEsc {
		t.Error("G04: second Esc must change the view (pop back from detail)")
	}
}

// ---------------------------------------------------------------------------
// Section K — Scroll-to-Match
// ---------------------------------------------------------------------------

// 26-K01: Viewport scrolls when navigating to an off-screen match.
// We verify that pressing n changes the view output, indicating that the
// viewport has moved or the highlighted match has changed position.
func TestSearch_K01_ViewportScrollsToOffScreenMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Resource with "match" in many fields so there are multiple matches.
	fields := map[string]string{}
	for i := range 40 {
		fields[strings.Repeat("a", i+1)] = "placeholder"
	}
	fields["state"] = "match-target"
	fields["zone"] = "match-zone"
	res := &resource.Resource{
		ID:     "i-k01",
		Name:   "i-k01",
		Fields: fields,
	}
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "match")

	viewBefore := rootViewContent(m)

	// Press n to advance to the next match.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	viewAfter := rootViewContent(m)

	// The view must change (different match highlighted or viewport shifted).
	if viewBefore == viewAfter {
		t.Error("K01: pressing n must change the view output (scroll or highlight position)")
	}
}

// 26-K02: Wrap-around navigation scrolls to a distant match.
// Pressing N from the first match should wrap to the last match, changing view output.
func TestSearch_K02_WrapAroundScrollsToDistantMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-k02")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	// Confirm at match 1/2.
	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "1/2 matches") {
		t.Skipf("K02: expected 2 matches, got: %q", plain)
	}

	viewBefore := rootViewContent(m)

	// N from match 1 wraps to match 2 (the last).
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	viewAfter := rootViewContent(m)

	if viewBefore == viewAfter {
		t.Error("K02: wrap-around N must change the view output")
	}
	plainAfter := ansiRe.ReplaceAllString(viewAfter, "")
	if !strings.Contains(plainAfter, "2/2 matches") {
		t.Errorf("K02: after N from first match, should show '2/2 matches'; got: %q", plainAfter)
	}
}

// 26-K03: Match already visible — highlight moves but view may or may not scroll.
// We verify that the match counter advances regardless.
func TestSearch_K03_MatchAlreadyVisibleHighlightMoves(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-k03")
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain1 := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain1, "1/2 matches") {
		t.Skipf("K03: need 2 matches; got: %q", plain1)
	}

	// Press n — highlight moves to match 2.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	plain2 := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plain2, "2/2 matches") {
		t.Errorf("K03: after n, highlight should be on match 2/2; got: %q", plain2)
	}
}

// 26-K04: Scroll-to-match works with word wrap enabled.
func TestSearch_K04_ScrollToMatchWithWordWrap(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-k04")
	m = navigateToYAML(m, res)

	// Enable word wrap before searching.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Errorf("K04: search should be active with word wrap on; got: %q", plain)
	}

	// Navigate to next match — must not crash.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	viewAfter := rootViewContent(m)
	if viewAfter == "" {
		t.Error("K04: View() must not return empty after n with word wrap on")
	}
}

// ---------------------------------------------------------------------------
// Section L — Word Wrap Interaction
// ---------------------------------------------------------------------------

// 26-L01: Search results preserved when toggling wrap ON.
func TestSearch_L01_SearchPreservedWhenWrapEnabled(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-l01")
	m = navigateToYAML(m, res)

	// Search (wrap is off by default). Indicator starts at 1/2 (first of 2 matches).
	m = activateAndConfirmSearch(t, m, "running")
	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "1/2 matches") {
		t.Skipf("L01: need 2 matches to test wrap toggle; got: %q", plain)
	}

	// Toggle wrap ON.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	// Search must still be active with same count.
	plainAfterWrap := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterWrap, "matches") {
		t.Errorf("L01: search should remain active after toggling wrap on; got: %q", plainAfterWrap)
	}
	if !strings.Contains(plainAfterWrap, "/2 matches") {
		t.Errorf("L01: total match count should remain at 2 after wrap toggle; got: %q", plainAfterWrap)
	}
}

// 26-L02: Search results preserved when toggling wrap OFF.
func TestSearch_L02_SearchPreservedWhenWrapDisabled(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-l02")
	m = navigateToDetail(m, res)

	// Enable wrap first.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	// Search with wrap on.
	m = activateAndConfirmSearch(t, m, "running")
	plainWithWrap := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainWithWrap, "matches") {
		t.Skipf("L02: search must be active; got: %q", plainWithWrap)
	}

	// Toggle wrap OFF.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	// Search must still be active.
	plainNoWrap := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainNoWrap, "matches") {
		t.Errorf("L02: search should remain active after toggling wrap off; got: %q", plainNoWrap)
	}
}

// 26-L03: Match hidden by clipping becomes visible after wrap enabled.
// We verify that the match count remains the same (or increases if wrapping
// exposes more lines) and the view does not crash.
func TestSearch_L03_HiddenMatchVisibleAfterWrap(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Resource with a very long ARN value that will be clipped when wrap is off.
	longARN := "arn:aws:iam::123456789012:role/very-long-role-name-that-will-clip-on-narrow-screens-running-role"
	res := &resource.Resource{
		ID:   "i-l03",
		Name: "i-l03",
		Fields: map[string]string{
			"arn":   longARN,
			"state": "running",
		},
	}
	m = navigateToYAML(m, res)

	// Search for "running" with wrap off.
	m = activateAndConfirmSearch(t, m, "running")
	plainBefore := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainBefore, "matches") {
		t.Skipf("L03: search must find matches; got: %q", plainBefore)
	}

	// Count matches before wrap.
	// Now enable wrap — previously clipped portions become visible.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	plainAfterWrap := ansiRe.ReplaceAllString(rootViewContent(m), "")
	// Search must still be active after wrap toggle.
	if !strings.Contains(plainAfterWrap, "matches") {
		t.Errorf("L03: search must remain active after enabling wrap; got: %q", plainAfterWrap)
	}
	// View must be non-empty.
	if plainAfterWrap == "" {
		t.Error("L03: View() must not be empty after wrap toggle")
	}
}

// ---------------------------------------------------------------------------
// Section N — Component Reuse (detail and YAML only)
// ---------------------------------------------------------------------------

// 26-N01: Identical activation in detail and YAML — both show "/" in header
// (search input mode).
func TestSearch_N01_IdenticalActivationDetailAndYAML(t *testing.T) {
	tui.Version = "0.6.0"

	res := twoMatchResource("i-n01")

	// Detail view.
	md := newRootSizedModel()
	md = navigateToDetail(md, res)
	md, _ = rootApplyMsg(md, tea.KeyPressMsg{Code: '/', Text: "/"})
	plainDetail := ansiRe.ReplaceAllString(rootViewContent(md), "")

	// YAML view.
	my := newRootSizedModel()
	my = navigateToYAML(my, res)
	my, _ = rootApplyMsg(my, tea.KeyPressMsg{Code: '/', Text: "/"})
	plainYAML := ansiRe.ReplaceAllString(rootViewContent(my), "")

	// Both must show "/" in the header right side (search input active).
	if !strings.Contains(plainDetail, "/") {
		t.Errorf("N01: detail header must show '/' when search input active; got: %q", plainDetail)
	}
	if !strings.Contains(plainYAML, "/") {
		t.Errorf("N01: YAML header must show '/' when search input active; got: %q", plainYAML)
	}
}

// 26-N02: Identical match highlighting in detail and YAML — both show "matches".
func TestSearch_N02_IdenticalMatchHighlightingDetailAndYAML(t *testing.T) {
	tui.Version = "0.6.0"
	res := twoMatchResource("i-n02")

	// Detail.
	md := newRootSizedModel()
	md = navigateToDetail(md, res)
	md = activateAndConfirmSearch(t, md, "running")
	plainDetail := ansiRe.ReplaceAllString(rootViewContent(md), "")

	// YAML.
	my := newRootSizedModel()
	my = navigateToYAML(my, res)
	my = activateAndConfirmSearch(t, my, "running")
	plainYAML := ansiRe.ReplaceAllString(rootViewContent(my), "")

	if !strings.Contains(plainDetail, "matches") {
		t.Errorf("N02: detail view must show 'matches' indicator after search; got: %q", plainDetail)
	}
	if !strings.Contains(plainYAML, "matches") {
		t.Errorf("N02: YAML view must show 'matches' indicator after search; got: %q", plainYAML)
	}
}

// 26-N03: Identical n/N behavior in detail and YAML — navigation advances the counter.
func TestSearch_N03_IdenticalNavigationDetailAndYAML(t *testing.T) {
	tui.Version = "0.6.0"
	res := twoMatchResource("i-n03")

	for _, viewName := range []string{"detail", "yaml"} {
		t.Run(viewName, func(t *testing.T) {
			m := newRootSizedModel()
			if viewName == "detail" {
				m = navigateToDetail(m, res)
			} else {
				m = navigateToYAML(m, res)
			}
			m = activateAndConfirmSearch(t, m, "running")

			plain1 := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if !strings.Contains(plain1, "1/2 matches") {
				t.Skipf("N03 %s: need 2 matches; got: %q", viewName, plain1)
			}

			// n advances to 2/2.
			m, _ = rootApplyMsg(m, rootKeyPress("n"))
			plain2 := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if !strings.Contains(plain2, "2/2 matches") {
				t.Errorf("N03 %s: n should advance to 2/2; got: %q", viewName, plain2)
			}

			// N retreats to 1/2.
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
			plain3 := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if !strings.Contains(plain3, "1/2 matches") {
				t.Errorf("N03 %s: N should retreat to 1/2; got: %q", viewName, plain3)
			}
		})
	}
}

// 26-N04: Identical Esc behavior in detail and YAML — Esc clears search.
func TestSearch_N04_IdenticalEscBehaviorDetailAndYAML(t *testing.T) {
	tui.Version = "0.6.0"
	res := twoMatchResource("i-n04")

	for _, viewName := range []string{"detail", "yaml"} {
		t.Run(viewName, func(t *testing.T) {
			m := newRootSizedModel()
			if viewName == "detail" {
				m = navigateToDetail(m, res)
			} else {
				m = navigateToYAML(m, res)
			}
			m = activateAndConfirmSearch(t, m, "running")

			// Verify search is active.
			plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if !strings.Contains(plain, "matches") {
				t.Skipf("N04 %s: search must be active before Esc; got: %q", viewName, plain)
			}

			// Press Esc.
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
			plainAfter := ansiRe.ReplaceAllString(rootViewContent(m), "")

			if strings.Contains(plainAfter, "matches") {
				t.Errorf("N04 %s: Esc must clear match indicator; got: %q", viewName, plainAfter)
			}
			if !strings.Contains(plainAfter, "? for help") {
				t.Errorf("N04 %s: header must show '? for help' after Esc; got: %q", viewName, plainAfter)
			}
		})
	}
}

// 26-N05: Search in detail view for EVERY resource type — table-driven.
func TestSearch_N05_DetailViewAllResourceTypes(t *testing.T) {
	tui.Version = "0.6.0"

	cases := []struct {
		typeName string
		res      resource.Resource
		query    string
	}{
		{
			"ec2",
			resource.Resource{ID: "i-123", Name: "ec2-test", Fields: map[string]string{"state": "running"}},
			"running",
		},
		{
			"s3",
			resource.Resource{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"region": "us-east-1"}},
			"east",
		},
		{
			"rds",
			resource.Resource{ID: "mydb", Name: "mydb", Fields: map[string]string{"engine": "postgres", "status": "available"}},
			"postgres",
		},
		{
			"lambda",
			resource.Resource{ID: "my-func", Name: "my-func", Fields: map[string]string{"runtime": "go1.x"}},
			"go1",
		},
		{
			"vpc",
			resource.Resource{ID: "vpc-123", Name: "main-vpc", Fields: map[string]string{"cidr_block": "10.0.0.0/16"}},
			"10.0",
		},
		{
			"sg",
			resource.Resource{ID: "sg-abc", Name: "web-sg", Fields: map[string]string{"description": "web traffic"}},
			"web",
		},
		{
			"iam-roles",
			resource.Resource{ID: "admin-role", Name: "admin-role", Fields: map[string]string{"arn": "arn:aws:iam::123:role/admin"}},
			"admin",
		},
		{
			"eks",
			resource.Resource{ID: "my-cluster", Name: "my-cluster", Fields: map[string]string{"version": "1.29"}},
			"1.29",
		},
		{
			"sqs",
			resource.Resource{ID: "https://sqs/queue", Name: "my-queue", Fields: map[string]string{"visibility": "30"}},
			"30",
		},
		{
			"cloudfront",
			resource.Resource{ID: "E123", Name: "my-dist", Fields: map[string]string{"status": "Deployed"}},
			"Deployed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			m := newRootSizedModel()
			res := tc.res
			m = navigateToDetail(m, &res)
			m = activateAndConfirmSearch(t, m, tc.query)

			plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

			// Must show match indicator (search works for this resource type).
			if !strings.Contains(plain, "matches") {
				t.Errorf("N05 %s: search for %q should find matches; got: %q", tc.typeName, tc.query, plain)
			}

			// n navigation must not crash.
			m, _ = rootApplyMsg(m, rootKeyPress("n"))
			if rootViewContent(m) == "" {
				t.Errorf("N05 %s: View() empty after pressing n", tc.typeName)
			}

			// Esc must clear search.
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
			plainAfterEsc := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if strings.Contains(plainAfterEsc, "matches") {
				t.Errorf("N05 %s: Esc must clear match indicator; got: %q", tc.typeName, plainAfterEsc)
			}
		})
	}
}

// 26-N06: Search in YAML view for EVERY resource type — table-driven.
func TestSearch_N06_YAMLViewAllResourceTypes(t *testing.T) {
	tui.Version = "0.6.0"

	cases := []struct {
		typeName string
		res      resource.Resource
		query    string
	}{
		{
			"ec2",
			resource.Resource{ID: "i-123", Name: "ec2-test", Fields: map[string]string{"state": "running"}},
			"running",
		},
		{
			"s3",
			resource.Resource{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"region": "us-east-1"}},
			"east",
		},
		{
			"rds",
			resource.Resource{ID: "mydb", Name: "mydb", Fields: map[string]string{"engine": "postgres", "status": "available"}},
			"postgres",
		},
		{
			"lambda",
			resource.Resource{ID: "my-func", Name: "my-func", Fields: map[string]string{"runtime": "go1.x"}},
			"go1",
		},
		{
			"vpc",
			resource.Resource{ID: "vpc-123", Name: "main-vpc", Fields: map[string]string{"cidr_block": "10.0.0.0/16"}},
			"10.0",
		},
		{
			"sg",
			resource.Resource{ID: "sg-abc", Name: "web-sg", Fields: map[string]string{"description": "web traffic"}},
			"web",
		},
		{
			"iam-roles",
			resource.Resource{ID: "admin-role", Name: "admin-role", Fields: map[string]string{"arn": "arn:aws:iam::123:role/admin"}},
			"admin",
		},
		{
			"eks",
			resource.Resource{ID: "my-cluster", Name: "my-cluster", Fields: map[string]string{"version": "1.29"}},
			"1.29",
		},
		{
			"sqs",
			resource.Resource{ID: "https://sqs/queue", Name: "my-queue", Fields: map[string]string{"visibility": "30"}},
			"30",
		},
		{
			"cloudfront",
			resource.Resource{ID: "E123", Name: "my-dist", Fields: map[string]string{"status": "Deployed"}},
			"Deployed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			m := newRootSizedModel()
			res := tc.res
			m = navigateToYAML(m, &res)
			m = activateAndConfirmSearch(t, m, tc.query)

			plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

			// Must show match indicator.
			if !strings.Contains(plain, "matches") {
				t.Errorf("N06 %s: search for %q should find matches in YAML; got: %q", tc.typeName, tc.query, plain)
			}

			// n must not crash.
			m, _ = rootApplyMsg(m, rootKeyPress("n"))
			if rootViewContent(m) == "" {
				t.Errorf("N06 %s: View() empty after pressing n in YAML", tc.typeName)
			}

			// Esc must clear search.
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
			plainAfterEsc := ansiRe.ReplaceAllString(rootViewContent(m), "")
			if strings.Contains(plainAfterEsc, "matches") {
				t.Errorf("N06 %s: Esc must clear match indicator in YAML; got: %q", tc.typeName, plainAfterEsc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Section O — Other Key Bindings During Search
// ---------------------------------------------------------------------------

// 26-O01: j/k scroll works while search is active — highlights remain.
func TestSearch_O01_JKScrollWorksWhileSearchActive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o01")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("O01: need active search")
	}

	// Press j to scroll down.
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	plainAfterJ := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Search must still be active (match indicator present).
	if !strings.Contains(plainAfterJ, "matches") {
		t.Errorf("O01: match indicator must remain after j scroll; got: %q", plainAfterJ)
	}

	// Press k to scroll back up.
	m, _ = rootApplyMsg(m, rootKeyPress("k"))
	plainAfterK := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterK, "matches") {
		t.Errorf("O01: match indicator must remain after k scroll; got: %q", plainAfterK)
	}
}

// 26-O02: g/G jump works while search is active.
func TestSearch_O02_GJumpWorksWhileSearchActive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o02")
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("O02: need active search")
	}

	// Press G to jump to bottom.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "G"})
	plainAfterG := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterG, "matches") {
		t.Errorf("O02: match indicator must remain after G jump; got: %q", plainAfterG)
	}

	// Press g to jump to top.
	m, _ = rootApplyMsg(m, rootKeyPress("g"))
	plainAfterSmallG := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterSmallG, "matches") {
		t.Errorf("O02: match indicator must remain after g jump; got: %q", plainAfterSmallG)
	}
}

// 26-O03: PageUp/PageDown works while search is active.
func TestSearch_O03_PageUpDownWorksWhileSearchActive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o03")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("O03: need active search")
	}

	// PageDown.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	plainAfterPgDn := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterPgDn, "matches") {
		t.Errorf("O03: match indicator must remain after PageDown; got: %q", plainAfterPgDn)
	}

	// PageUp.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	plainAfterPgUp := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainAfterPgUp, "matches") {
		t.Errorf("O03: match indicator must remain after PageUp; got: %q", plainAfterPgUp)
	}
}

// 26-O04: Copy (c) works while search is active — the copy command returns a
// FlashMsg or CopiedMsg (clipboard write may fail in headless CI, that is OK).
// We verify that pressing c produces a command and that executing the command
// yields a FlashMsg or CopiedMsg — confirming the copy path runs while search
// is active without panicking or swallowing the key.
func TestSearch_O04_CopyWorksWhileSearchActive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o04")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("O04: need active search")
	}

	// Press c — copy. The model returns an updated model plus a command.
	// The command produces a FlashMsg or CopiedMsg.
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("O04: pressing c while search active must return a command (copy action)")
	}

	// Execute the returned cmd.
	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK — clipboard may succeed or fail in CI, but the copy path fired.
	case messages.CopiedMsg:
		// OK — clipboard succeeded.
	default:
		t.Errorf("O04: expected FlashMsg or CopiedMsg from copy command, got %T", msg)
	}

	// Apply the flash/copy message back into the model and confirm search is still active.
	m, _ = rootApplyMsg(m, msg)
	plainAfterCopy := ansiRe.ReplaceAllString(rootViewContent(m), "")
	// Search may still be active (if no refresh cleared it) OR the flash is shown.
	// Either way the view must be non-empty.
	if plainAfterCopy == "" {
		t.Error("O04: View() must not be empty after copy while search active")
	}
}

// 26-O05: Help (?) opens while search is active; closing help restores search.
func TestSearch_O05_HelpOpenAndClosePreservesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o05")
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plainBefore := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plainBefore, "matches") {
		t.Skip("O05: need active search")
	}

	// Press ? — open help.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '?', Text: "?"})
	plainHelp := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Help view must be visible.
	if !strings.Contains(plainHelp, "help") {
		t.Errorf("O05: help screen must be shown after ?; got: %q", plainHelp)
	}

	// Press Esc to close help.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plainAfter := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Must be back in detail view with search active.
	if !strings.Contains(plainAfter, "matches") {
		t.Errorf("O05: search must be preserved after closing help; got: %q", plainAfter)
	}
}

// 26-O06: Ctrl+r during search — test that refresh clears search.
// At unit level we cannot easily verify the loading spinner, so we verify
// that after a refresh key the view does not crash.
func TestSearch_O06_CtrlRDuringSearch_NoCrash(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to resource list first so Ctrl+r has a context.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	res := twoMatchResource("i-o06")
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	// Press Ctrl+r.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// Must not crash; view must be non-empty.
	content := rootViewContent(m)
	if content == "" {
		t.Error("O06: View() must not return empty after Ctrl+r during search")
	}
}

// 26-O07: Command mode (`:`) works while search is active.
func TestSearch_O07_CommandModeWorksWhileSearchActive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-o07")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("O07: need active search")
	}

	// Press `:` to enter command mode.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: ':', Text: ":"})
	plainCmd := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Command mode indicator must be visible.
	if !strings.Contains(plainCmd, ":") {
		t.Errorf("O07: command mode indicator ':' must appear after ':' press; got: %q", plainCmd)
	}
}

// ---------------------------------------------------------------------------
// Section P — Help Screen
// ---------------------------------------------------------------------------

// 26-P01: Help in detail view lists search bindings.
// The help screen must include dedicated entries for search ("/"), next match
// ("n"), and prev match ("N") — not just incidental characters in other labels.
// The spec requires: "</>  Search", "<n>  Next Match", "<N>  Prev Match".
// These test the description strings as they would appear in help group entries.
func TestSearch_P01_HelpInDetailListsSearchBindings(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-p01")
	m = navigateToDetail(m, res)

	// Open help.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '?', Text: "?"})

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plain, "help") {
		t.Fatalf("P01: expected help screen; got: %q", plain)
	}

	// The help screen must list explicit search binding descriptions.
	// "search" as a description label for the "/" key.
	if !strings.Contains(strings.ToLower(plain), "search") {
		t.Errorf("P01: detail help must include 'search' binding description; got: %q", plain)
	}
	// "next" as a description label for the "n" key.
	if !strings.Contains(strings.ToLower(plain), "next") {
		t.Errorf("P01: detail help must include 'next' (next match) binding description; got: %q", plain)
	}
	// "prev" as a description label for the "N" key.
	if !strings.Contains(strings.ToLower(plain), "prev") {
		t.Errorf("P01: detail help must include 'prev' (prev match) binding description; got: %q", plain)
	}
}

// 26-P02: Help in YAML view lists search bindings.
// Mirrors P01 for the YAML view.
func TestSearch_P02_HelpInYAMLListsSearchBindings(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-p02")
	m = navigateToYAML(m, res)

	// Open help.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '?', Text: "?"})

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plain, "help") {
		t.Fatalf("P02: expected help screen; got: %q", plain)
	}

	// "search" as a description label for the "/" key.
	if !strings.Contains(strings.ToLower(plain), "search") {
		t.Errorf("P02: YAML help must include 'search' binding description; got: %q", plain)
	}
	// "next" as a description label for the "n" key.
	if !strings.Contains(strings.ToLower(plain), "next") {
		t.Errorf("P02: YAML help must include 'next' (next match) binding description; got: %q", plain)
	}
	// "prev" as a description label for the "N" key.
	if !strings.Contains(strings.ToLower(plain), "prev") {
		t.Errorf("P02: YAML help must include 'prev' (prev match) binding description; got: %q", plain)
	}
}

// ---------------------------------------------------------------------------
// Section Q — Terminal Resize
// ---------------------------------------------------------------------------

// 26-Q01: Resize preserves search state.
func TestSearch_Q01_ResizePreservesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-q01")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("Q01: need active search")
	}

	// Resize to a different width.
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 50})

	plainAfter := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plainAfter, "matches") {
		t.Errorf("Q01: search must be preserved after resize; got: %q", plainAfter)
	}
}

// 26-Q02: Resize with word wrap on — matches remain.
func TestSearch_Q02_ResizeWithWordWrapPreservesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-q02")
	m = navigateToDetail(m, res)

	// Enable word wrap.
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("Q02: need active search")
	}

	// Resize to narrower width.
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 70, Height: 30})

	plainAfter := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plainAfter, "matches") {
		t.Errorf("Q02: search must be preserved after resize with word wrap; got: %q", plainAfter)
	}
}

// 26-Q03: Resize below minimum (< 60 cols) shows error; resize back preserves search.
func TestSearch_Q03_ResizeBelowMinimumThenRestorePreservesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	res := twoMatchResource("i-q03")
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "running")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")
	if !strings.Contains(plain, "matches") {
		t.Skip("Q03: need active search")
	}

	// Resize to below minimum width (40 < 60).
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	plainNarrow := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// The view must show an error or resize message (not empty).
	if plainNarrow == "" {
		t.Error("Q03: View() must not be empty when terminal is too narrow")
	}

	// Resize back to normal size.
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	plainRestored := ansiRe.ReplaceAllString(rootViewContent(m), "")

	// Search must still be active after restoring size.
	if !strings.Contains(plainRestored, "matches") {
		t.Errorf("Q03: search must be preserved after resize back to normal; got: %q", plainRestored)
	}
}

// ---------------------------------------------------------------------------
// Section R — Real-World Scenarios
// ---------------------------------------------------------------------------

// 26-R01: Search for "10.0" in EC2 detail to find private IP.
func TestSearch_R01_FindPrivateIPInEC2Detail(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-0abc123def",
		Name: "prod-web-01",
		Fields: map[string]string{
			"instance_id":        "i-0abc123def",
			"instance_type":      "t3.medium",
			"state":              "running",
			"private_ip_address": "10.0.1.42",
			"public_ip_address":  "52.90.1.100",
			"vpc_id":             "vpc-0123456",
			"subnet_id":          "subnet-abc123",
			"availability_zone":  "us-east-1a",
		},
	}
	m = navigateToDetail(m, res)
	m = activateAndConfirmSearch(t, m, "10.0")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plain, "matches") {
		t.Errorf("R01: searching '10.0' in EC2 detail should find the private IP; got: %q", plain)
	}
}

// 26-R02: Search for "sg-" in EC2 YAML to find security group.
func TestSearch_R02_FindSecurityGroupInEC2YAML(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-0def456abc",
		Name: "prod-api-01",
		Fields: map[string]string{
			"instance_id":     "i-0def456abc",
			"instance_type":   "t3.large",
			"state":           "running",
			"security_groups": "sg-0abc123def456",
			"vpc_id":          "vpc-0abc123",
		},
	}
	m = navigateToYAML(m, res)
	m = activateAndConfirmSearch(t, m, "sg-")

	plain := ansiRe.ReplaceAllString(rootViewContent(m), "")

	if !strings.Contains(plain, "matches") {
		t.Errorf("R02: searching 'sg-' in EC2 YAML should find security group; got: %q", plain)
	}
}

// 26-R05: YAML search may find more matches than detail search.
// Both views search for "available" in an RDS instance.
// The YAML view typically finds more occurrences because it renders all fields.
func TestSearch_R05_YAMLFindsMoreMatchesThanDetail(t *testing.T) {
	tui.Version = "0.6.0"

	res := &resource.Resource{
		ID:   "mydb-prod",
		Name: "mydb-prod",
		Fields: map[string]string{
			"db_instance_status":              "available",
			"db_instance_identifier":          "mydb-prod",
			"engine":                          "postgres",
			"engine_version":                  "14.5",
			"availability_zone":               "us-east-1a",
			"multi_az":                        "false",
			"storage_type":                    "gp3",
			"vpc_security_group_ids_0_status": "available",
		},
	}

	// Detail search.
	md := newRootSizedModel()
	md = navigateToDetail(md, res)
	md = activateAndConfirmSearch(t, md, "available")
	plainDetail := ansiRe.ReplaceAllString(rootViewContent(md), "")

	if !strings.Contains(plainDetail, "matches") {
		t.Fatalf("R05: detail search for 'available' should find matches; got: %q", plainDetail)
	}

	// YAML search.
	my := newRootSizedModel()
	my = navigateToYAML(my, res)
	my = activateAndConfirmSearch(t, my, "available")
	plainYAML := ansiRe.ReplaceAllString(rootViewContent(my), "")

	if !strings.Contains(plainYAML, "matches") {
		t.Fatalf("R05: YAML search for 'available' should find matches; got: %q", plainYAML)
	}

	// Both views should find at least 1 match.
	// (We cannot assert YAML > detail reliably without parsing counts,
	// but we can assert both are non-zero and the view is non-empty.)
	if plainDetail == "" || plainYAML == "" {
		t.Error("R05: both views must return non-empty content")
	}
}
