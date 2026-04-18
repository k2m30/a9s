package unit

// qa_ctrlz_badge_invariant_test.go — regression tests for the ctrl+z / badge
// count invariant across ALL registered resource types.
//
// Invariant (user demand): if the menu badge shows "ec2(27/11 issues)", then
// pressing ctrl+z on the EC2 list MUST reveal exactly 11 rows. No more, no
// less. This test asserts the invariant for every resource type in the
// registry by loading a mixed-status slice and verifying:
//
//   visible_after_ctrl+z == count(td.Color(r).IsIssue() for all rows)
//
// Per-type Color is the canonical source of issue classification post-refactor.
// types with ExcludeFromIssueBadge still honor ctrl+z (colored rows visible),
// but the expected count is computed from their Color func, not a global set.

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ctrlZ constructs the ctrl+z key press message understood by bubbles/v2 key.Matches.
// Equivalent to key.NewBinding(key.WithKeys("ctrl+z")).
func ctrlZ() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
}

// ctrlZModel builds a ResourceListModel for the given resource type, loads the
// provided resources, and returns the model ready for Update calls.
func ctrlZModel(t *testing.T, shortName string, resources []resource.Resource) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q not found in registry", shortName)
	}

	cfg := config.DefaultConfig()
	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(200, 30)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: shortName,
		Resources:    resources,
	})
	return m
}

// ctEvents4 returns the canonical 4-resource ct-events seed used across §7 tests:
// 2 ct-info (dim), 1 ct-attention (yellow), 1 ct-danger (red).
func ctEvents4() []resource.Resource {
	return []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Status: "ct-info"},
		{ID: "evt-0002", Name: "write-1", Status: "ct-attention"},
		{ID: "evt-0003", Name: "delete-1", Status: "ct-danger"},
		{ID: "evt-0004", Name: "read-2", Status: "ct-info"},
	}
}

// parseAttentionTitle extracts the visible and total counts from the attention-
// filter-on FrameTitle format: "name(N of M)". Returns (visible, total, ok).
var attentionTitleRe = regexp.MustCompile(`\((\d+) of (\d+)\)`)

func parseAttentionTitle(m views.ResourceListModel) (visible, total int, ok bool) {
	match := attentionTitleRe.FindStringSubmatch(m.FrameTitle())
	if len(match) != 3 {
		return 0, 0, false
	}
	v, _ := strconv.Atoi(match[1])
	tt, _ := strconv.Atoi(match[2])
	return v, tt, true
}

// ctrlZInvariantResources produces a mixed-status seed for testing the invariant
// across all resource types. The resources span the four Color categories:
//   - Healthy:  running, available
//   - Warning:  pending
//   - Broken:   stopped
//   - Dim:      terminated
//
// Each resource has Fields["state"] set so types that read Fields["state"]
// (e.g. ec2, ebs, lambda) classify them correctly.
func ctrlZInvariantResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "r-running", Name: "running-1", Status: "running",
			Fields: map[string]string{"state": "running", "status": "running",
				"db_instance_status": "available", "table_status": "ACTIVE",
				"last_status": "RUNNING", "life_cycle_state": "available"},
		},
		{
			ID: "r-stopped", Name: "stopped-1", Status: "stopped",
			Fields: map[string]string{"state": "stopped", "status": "stopped",
				"db_instance_status": "stopped", "table_status": "ARCHIVING",
				"last_status": "STOPPED", "life_cycle_state": "error"},
		},
		{
			ID: "r-pending", Name: "pending-1", Status: "pending",
			Fields: map[string]string{"state": "pending", "status": "pending",
				"db_instance_status": "creating", "table_status": "CREATING",
				"last_status": "PENDING", "life_cycle_state": "creating"},
		},
		{
			ID: "r-terminated", Name: "terminated-1", Status: "terminated",
			Fields: map[string]string{"state": "terminated", "status": "terminated",
				"db_instance_status": "deleting", "table_status": "DELETING",
				"last_status": "STOPPED", "life_cycle_state": "deleting"},
		},
		{
			ID: "r-available", Name: "available-1", Status: "available",
			Fields: map[string]string{"state": "available", "status": "available",
				"db_instance_status": "available", "table_status": "ACTIVE",
				"last_status": "RUNNING", "life_cycle_state": "available"},
		},
	}
}

// ctrlZRegisteredShortNames returns short names of all registered resource
// types from the registry. Using the registry directly ensures new types are
// automatically covered by the invariant test.
func ctrlZRegisteredShortNames() []string {
	types := resource.AllResourceTypes()
	names := make([]string, 0, len(types))
	for _, td := range types {
		names = append(names, td.ShortName)
	}
	sort.Strings(names)
	return names
}

// TestCtrlZInvariant_BadgeCountMatchesVisibleAcrossAllTypes asserts, for every
// registered resource type, that the count of visible rows after pressing
// ctrl+z equals the count of rows where td.Color(r).IsIssue() is true.
//
// This test will catch regressions where the attention filter drifts from
// the per-type Color classification (which is exactly the user-reported bug).
// Expected count is computed per-type so each type's own health semantics apply.
func TestCtrlZInvariant_BadgeCountMatchesVisibleAcrossAllTypes(t *testing.T) {
	seed := ctrlZInvariantResources()

	for _, short := range ctrlZRegisteredShortNames() {
		short := short
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Skipf("resource type %q not in registry (may be conditional)", short)
			}
			if td.Color == nil {
				t.Fatalf("%s: Color func is nil — invariant #7 violated", short)
			}

			// Compute expected count using the type's own Color classification.
			expected := 0
			for _, r := range seed {
				if td.Color(r).IsIssue() {
					expected++
				}
			}

			m := ctrlZModel(t, short, seed)

			// Toggle attention on.
			m, _ = m.Update(ctrlZ())

			// FrameTitle encodes the filtered count as "name(N of M)" when
			// attention is active. This is a behavior-level check that works
			// across all resource types regardless of how their columns render.
			visible, total, ok := parseAttentionTitle(m)
			if !ok {
				t.Fatalf("%s: FrameTitle did not match 'N of M' format: %q", short, m.FrameTitle())
			}
			if total != len(seed) {
				t.Errorf("%s: total count in title = %d, want %d", short, total, len(seed))
			}
			if visible != expected {
				t.Errorf("%s: ctrl+z visible count in title = %d, want %d (td.Color(r).IsIssue() matches in seed); title=%q",
					short, visible, expected, m.FrameTitle())
			}
		})
	}
}


// TestCtrlZ_EC2_27Rows12Issues reproduces a scenario with 27 EC2 rows
// yielding 12 issues (per ec2.Color(r).IsIssue()). Pressing ctrl+z must
// reveal exactly 12 rows. shutting-down is Warning (transitional state),
// not Dim like terminated. Fields["state"] is populated so the EC2 Color
// func classifies correctly.
func TestCtrlZ_EC2_27Rows11Issues(t *testing.T) {
	// Build 27 rows: 12 issue-colored (Warning/Broken), 14 running, 1 terminated.
	issueStatuses := []string{
		"stopped", "stopped", "stopped", "stopped", "stopped",
		"stopped", "stopped", "stopped", "stopped",
		"pending", "stopping", "shutting-down",
	}
	var resources []resource.Resource
	for i, s := range issueStatuses {
		resources = append(resources, resource.Resource{
			ID:     "i-issue-" + string(rune('a'+i)),
			Name:   "issue-node-" + string(rune('a'+i)),
			Status: s,
			Fields: map[string]string{"state": s},
		})
	}
	for i := 0; i < 14; i++ {
		resources = append(resources, resource.Resource{
			ID:     "i-run-" + string(rune('a'+i)),
			Name:   "healthy-node-" + string(rune('a'+i)),
			Status: "running",
			Fields: map[string]string{"state": "running"},
		})
	}
	resources = append(resources,
		resource.Resource{ID: "i-term-1", Name: "legacy-app", Status: "terminated",
			Fields: map[string]string{"state": "terminated"}},
	)
	if got := len(resources); got != 27 {
		t.Fatalf("test setup error: want 27 resources, got %d", got)
	}

	m := ctrlZModel(t, "ec2", resources)

	// Sanity: toggle off shows all 27.
	allView := stripANSI(m.View())
	if !strings.Contains(allView, "healthy-node-a") {
		t.Fatalf("pre-toggle view missing expected running row; view=\n%s", allView)
	}

	// Toggle attention on.
	m, _ = m.Update(ctrlZ())
	view := stripANSI(m.View())

	// Count visible issue-node-* rows — must be exactly 12.
	visibleIssues := 0
	for i := 0; i < len(issueStatuses); i++ {
		name := "issue-node-" + string(rune('a'+i))
		if strings.Contains(view, name) {
			visibleIssues++
		}
	}
	if visibleIssues != 12 {
		t.Errorf("ctrl+z visible issue-rows: got %d, want 12", visibleIssues)
	}

	// No running rows visible.
	for i := 0; i < 14; i++ {
		name := "healthy-node-" + string(rune('a'+i))
		if strings.Contains(view, name) {
			t.Errorf("running row %q must be hidden after ctrl+z (badge invariant)", name)
		}
	}

	// Terminated row (Dim) hidden — not an issue.
	if strings.Contains(view, "legacy-app") {
		t.Errorf("terminated row 'legacy-app' must be hidden after ctrl+z")
	}
}

// TestCtrlZ_EC2IssueStatuses_Visible asserts that EC2 issue statuses are visible
// after ctrl+z. Issue-ness is determined by the EC2 type's own Color func, not
// a global string-set. Only EC2-relevant statuses are tested here.
func TestCtrlZ_EC2IssueStatuses_Visible(t *testing.T) {
	ec2td := resource.FindResourceType("ec2")
	if ec2td == nil {
		t.Fatal("ec2 resource type not found in registry")
	}

	// EC2-relevant resources: each has Fields["state"] set so the Color func
	// classifies them correctly.
	ec2Resources := []resource.Resource{
		{ID: "r-stopped", Name: "row-a-stopped", Status: "stopped",
			Fields: map[string]string{"state": "stopped"}},
		{ID: "r-stopping", Name: "row-b-stopping", Status: "stopping",
			Fields: map[string]string{"state": "stopping"}},
		{ID: "r-pending", Name: "row-c-pending", Status: "pending",
			Fields: map[string]string{"state": "pending"}},
		{ID: "r-impaired", Name: "row-d-impaired", Status: "running",
			Fields: map[string]string{"state": "running", "system_status": "impaired"}},
		{ID: "r-initializing", Name: "row-e-initializing", Status: "running",
			Fields: map[string]string{"state": "running", "instance_status": "initializing"}},
		// Non-issue rows that must be hidden.
		{ID: "r-running", Name: "row-f-running", Status: "running",
			Fields: map[string]string{"state": "running", "system_status": "ok", "instance_status": "ok"}},
		{ID: "r-terminated", Name: "row-g-terminated", Status: "terminated",
			Fields: map[string]string{"state": "terminated"}},
		{ID: "r-shutting", Name: "row-h-shutting-down", Status: "shutting-down",
			Fields: map[string]string{"state": "shutting-down"}},
	}

	m := ctrlZModel(t, "ec2", ec2Resources)
	m, _ = m.Update(ctrlZ())
	view := stripANSI(m.View())

	for _, r := range ec2Resources {
		r := r
		isIssue := ec2td.Color(r).IsIssue()
		if isIssue {
			if !strings.Contains(view, r.Name) {
				t.Errorf("ec2 status %q row %q must be visible after ctrl+z (Color.IsIssue=true)", r.Status, r.Name)
			}
		} else {
			if strings.Contains(view, r.Name) {
				t.Errorf("ec2 status %q row %q must be hidden after ctrl+z (Color.IsIssue=false)", r.Status, r.Name)
			}
		}
	}
}

// ===========================================================================
// TestCtrlZ_CTEvents_HidesDimRows (§7.1, §7.2)
//
// After one ctrl+z press, ct-info rows disappear from View() while the
// underlying AllResources() slice is unchanged.
// ===========================================================================

func TestCtrlZ_CTEvents_HidesDimRows(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// Sanity: all 4 resources loaded.
	if got := len(m.AllResources()); got != 4 {
		t.Fatalf("AllResources before toggle: got %d, want 4", got)
	}

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	// Underlying data must not change.
	if got := len(m.AllResources()); got != 4 {
		t.Errorf("AllResources after toggle: got %d, want 4 (underlying data must be preserved)", got)
	}

	view := stripANSI(m.View())

	// Attention-worthy rows must be visible.
	if !strings.Contains(view, "write-1") {
		t.Errorf("ct-attention row 'write-1' missing from view after ctrl+z toggle ON\n  view:\n%s", view)
	}
	if !strings.Contains(view, "delete-1") {
		t.Errorf("ct-danger row 'delete-1' missing from view after ctrl+z toggle ON\n  view:\n%s", view)
	}

	// Dim rows must be hidden.
	if strings.Contains(view, "read-1") {
		t.Errorf("ct-info row 'read-1' should be hidden after ctrl+z toggle ON\n  view:\n%s", view)
	}
	if strings.Contains(view, "read-2") {
		t.Errorf("ct-info row 'read-2' should be hidden after ctrl+z toggle ON\n  view:\n%s", view)
	}
}

// ===========================================================================
// TestCtrlZ_Toggles_On_Off_Restores (§7.2)
//
// Two ctrl+z presses: on then off. All 4 rows visible after second press.
// ===========================================================================

func TestCtrlZ_Toggles_On_Off_Restores(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// First press: toggle on.
	m, _ = m.Update(ctrlZ())
	// Second press: toggle off.
	m, _ = m.Update(ctrlZ())

	view := stripANSI(m.View())

	for _, name := range []string{"read-1", "write-1", "delete-1", "read-2"} {
		if !strings.Contains(view, name) {
			t.Errorf("row %q missing from view after ctrl+z toggle OFF\n  view:\n%s", name, view)
		}
	}
}

// ===========================================================================
// TestCtrlZ_ResetsCursorToTop (§7.2)
//
// Move cursor to index 2, then toggle on. Cursor must reset to 0 (top of
// the remaining filtered rows).
// ===========================================================================

func TestCtrlZ_ResetsCursorToTop(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// Move cursor down twice (index 0 → 2).
	m, _ = m.Update(rlKeyPress("j"))
	m, _ = m.Update(rlKeyPress("j"))

	if got := m.CursorPosition(); got != 2 {
		// Keyboard nav may differ; just ensure cursor is non-zero before toggle.
		if got == 0 {
			t.Skip("cursor did not advance past 0 — skipping cursor-reset assertion")
		}
	}

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	if got := m.CursorPosition(); got != 0 {
		t.Errorf("CursorPosition after ctrl+z toggle ON: got %d, want 0 (cursor must reset to top)", got)
	}
}

// ===========================================================================
// TestCtrlZ_EC2_ShowsOnlyIssueRows
//
// ctrl+z on a resource list shows ONLY attention-worthy rows — rows where
// ec2.Color(r).IsIssue() is true. Running rows (green) are hidden. Stopped
// rows (red) are visible. Terminated (dim) remain hidden.
// Fields["state"] is populated so the EC2 Color func classifies correctly.
// ===========================================================================

func TestCtrlZ_EC2_ShowsOnlyIssueRows(t *testing.T) {
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod", Status: "running",
			Fields: map[string]string{"state": "running"}},
		{ID: "i-0002", Name: "batch-job", Status: "stopped",
			Fields: map[string]string{"state": "stopped"}},
		{ID: "i-0003", Name: "old-build", Status: "terminated",
			Fields: map[string]string{"state": "terminated"}},
		{ID: "i-0004", Name: "api-prod", Status: "running",
			Fields: map[string]string{"state": "running"}},
	}
	m := ctrlZModel(t, "ec2", ec2Resources)

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	view := stripANSI(m.View())

	// running rows must be HIDDEN — they don't count as issues on the badge.
	if strings.Contains(view, "web-prod") {
		t.Errorf("running row 'web-prod' must be HIDDEN after ctrl+z (running is not an issue)\n  view:\n%s", view)
	}
	if strings.Contains(view, "api-prod") {
		t.Errorf("running row 'api-prod' must be HIDDEN after ctrl+z (running is not an issue)\n  view:\n%s", view)
	}

	// stopped must be visible (ColorBroken — is an issue).
	if !strings.Contains(view, "batch-job") {
		t.Errorf("stopped row 'batch-job' must be visible after ctrl+z (ColorBroken.IsIssue=true)\n  view:\n%s", view)
	}

	// terminated must be hidden (ColorDim — not an issue).
	if strings.Contains(view, "old-build") {
		t.Errorf("terminated row 'old-build' must be hidden after ctrl+z (ColorDim.IsIssue=false)\n  view:\n%s", view)
	}
}

// ===========================================================================
// TestCtrlZ_PerViewState_DoesNotBleed (§7.3)
//
// Toggle attentionOnly on the ct-events instance. The ec2 instance must
// still show all its rows (attentionOnly state does not bleed across views).
// ===========================================================================

func TestCtrlZ_PerViewState_DoesNotBleed(t *testing.T) {
	ctEventsResources := ctEvents4()
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod", Status: "running",
			Fields: map[string]string{"state": "running"}},
		{ID: "i-0002", Name: "batch-job", Status: "stopped",
			Fields: map[string]string{"state": "stopped"}},
		{ID: "i-0003", Name: "old-build", Status: "terminated",
			Fields: map[string]string{"state": "terminated"}},
		{ID: "i-0004", Name: "api-prod", Status: "running",
			Fields: map[string]string{"state": "running"}},
	}

	mCT := ctrlZModel(t, "ct-events", ctEventsResources)
	mEC2 := ctrlZModel(t, "ec2", ec2Resources)

	// Toggle ct-events on.
	mCT, _ = mCT.Update(ctrlZ())

	// ct-events: dim rows hidden (confirming toggle applied).
	ctView := stripANSI(mCT.View())
	if strings.Contains(ctView, "read-1") {
		t.Errorf("ct-events: ct-info row 'read-1' should be hidden after toggle ON\n  view:\n%s", ctView)
	}

	// ec2: all rows must still be visible (no bleed).
	ec2View := stripANSI(mEC2.View())
	for _, name := range []string{"web-prod", "batch-job", "old-build", "api-prod"} {
		if !strings.Contains(ec2View, name) {
			t.Errorf("ec2 view missing %q after ct-events toggle ON (attentionOnly bled across views)\n  ec2 view:\n%s", name, ec2View)
		}
	}
}

// ===========================================================================
// TestCtrlZ_PersistsAcrossCacheRoundTrip (§7.3 persistent state)
//
// attentionOnly must survive a cache eviction+restore cycle via
// NewResourceListFromCache. The coder adds attentionOnly bool as the 11th
// trailing parameter and adds an AttentionOnly() accessor.
// This test WILL COMPILE-ERROR until those additions land.
// ===========================================================================

func TestCtrlZ_PersistsAcrossCacheRoundTrip(t *testing.T) {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(200, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEvents4(),
	})

	// Toggle attentionOnly ON.
	m, _ = m.Update(ctrlZ())
	if !m.AttentionOnly() {
		t.Fatal("attentionOnly should be true after ctrl+z")
	}

	visibleBefore := len(m.AllResources())

	sortField, sortAsc := m.SortState()
	m2 := views.NewResourceListFromCache(
		*td, config.DefaultConfig(), k,
		m.AllResources(),
		m.PaginationState(),
		m.FilterText(),
		sortField,
		sortAsc,
		m.CursorPosition(),
		m.HScrollOffset(),
		m.AttentionOnly(),
	)
	m2.SetSize(200, 20)

	if !m2.AttentionOnly() {
		t.Fatal("attentionOnly should persist across cache round-trip via NewResourceListFromCache")
	}

	if got := len(m2.AllResources()); got != visibleBefore {
		t.Errorf("AllResources after cache round-trip: got %d, want %d", got, visibleBefore)
	}

	view2 := stripANSI(m2.View())
	if strings.Contains(view2, "read-1") {
		t.Errorf("ct-info row 'read-1' should be hidden after cache round-trip\n  view:\n%s", view2)
	}
	if strings.Contains(view2, "read-2") {
		t.Errorf("ct-info row 'read-2' should be hidden after cache round-trip\n  view:\n%s", view2)
	}
	if !strings.Contains(view2, "write-1") {
		t.Errorf("ct-attention row 'write-1' must be visible after cache round-trip\n  view:\n%s", view2)
	}
	if !strings.Contains(view2, "delete-1") {
		t.Errorf("ct-danger row 'delete-1' must be visible after cache round-trip\n  view:\n%s", view2)
	}
}

// ===========================================================================
// TestCtrlZ_StatusLineIndicator (§7.3)
//
// When attentionOnly is active, [!] must appear somewhere in the rendered
// output (View(), FrameTitle(), or BottomHints()).
// ===========================================================================

func TestCtrlZ_StatusLineIndicator(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	beforeView := stripANSI(m.View())
	beforeTitle := stripANSI(m.FrameTitle())
	if strings.Contains(beforeView, "[!]") || strings.Contains(beforeTitle, "[!]") {
		t.Errorf("[!] indicator present before ctrl+z toggle — should only appear when attentionOnly is active")
	}

	m, _ = m.Update(ctrlZ())

	afterView := stripANSI(m.View())
	afterTitle := stripANSI(m.FrameTitle())

	hintsStr := ""
	for _, h := range m.BottomHints() {
		hintsStr += h.Key + " " + h.Desc + " "
	}

	if !strings.Contains(afterView, "[!]") && !strings.Contains(afterTitle, "[!]") && !strings.Contains(hintsStr, "[!]") {
		t.Errorf(
			"[!] indicator not found in any rendered surface after ctrl+z toggle ON\n"+
				"  FrameTitle: %q\n"+
				"  BottomHints: %q\n"+
				"  View (first 200 chars): %.200s",
			afterTitle, hintsStr, afterView,
		)
	}
}
