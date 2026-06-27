// menu_render_parity_test.go — byte-parity gate for PR-C 1b menu flip.
//
// Asserts that MainMenuModel.RenderBody(body) produces output byte-identical
// to the legacy MainMenuModel.View() for the same logical menu state.
//
// Strategy: for each scenario, build the SAME logical state on BOTH sides:
//   - Legacy side: drive MainMenuModel via SetAvailability/SetTruncated/SetIssues/
//     SetFilter/Toggle and navigate with Update(keyMsg) to align cursor+scroll.
//   - Controller side: drive an app.Controller via ApplyIntents and Apply(Action)
//     with the identical data, then read body := *controller.Snapshot().Body.Menu.
//   - Call m.RenderBody(body) on the SAME model m, because RenderBody reads
//     m.scrollOffset/m.width/m.height from the model — the viewport geometry
//     must match.
//   - Assert got == legacy EXACTLY (byte-parity). Any difference is a real
//     regression in RenderBody and must be reported, not suppressed.
//
// If a scenario fails this test reports the exact mismatch so the architect
// can fix RenderBody. Do NOT edit RenderBody to paper over failures.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newParityMenu creates a MainMenuModel with NO_COLOR and the given dimensions.
// It also constructs a matching Controller rooted at ScreenMenu.
// Both share NO_COLOR so styled output is deterministic.
func newParityPair(t *testing.T, w, h int) (views.MainMenuModel, *app.Controller) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(w, h)

	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)

	return m, c
}

// assertParity calls m.View() and m.RenderBody(body) and fails with an
// explicit character-level diff if the two strings differ.
func assertParity(t *testing.T, m *views.MainMenuModel, body app.MenuBody) {
	t.Helper()
	legacy := m.View()
	got := m.RenderBody(body)
	if got == legacy {
		return
	}
	// Produce an explicit diff so any byte difference is immediately visible.
	legacyLines := strings.Split(legacy, "\n")
	gotLines := strings.Split(got, "\n")
	maxLines := len(legacyLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf(
		"RenderBody output differs from View() — %d legacy lines vs %d RenderBody lines\n",
		len(legacyLines), len(gotLines),
	))
	for i := 0; i < maxLines; i++ {
		legLine, gotLine := "", ""
		if i < len(legacyLines) {
			legLine = legacyLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if legLine != gotLine {
			diff.WriteString(fmt.Sprintf(
				"  line %d differs:\n    legacy:     %q\n    RenderBody: %q\n",
				i+1, legLine, gotLine,
			))
		}
	}
	t.Errorf("byte-parity FAILED:\n%s", diff.String())
}

// firstNonExcludedShortName returns the ShortName of the first resource type
// that does NOT have ExcludeFromIssueBadge set. Used to populate issue badges
// without triggering the exclusion path.
func firstNonExcludedShortName(t *testing.T) string {
	t.Helper()
	for _, rt := range resource.AllResourceTypes() {
		if !rt.ExcludeFromIssueBadge {
			return rt.ShortName
		}
	}
	t.Fatal("no non-excluded resource type found in catalog")
	return ""
}

// firstExcludedShortName returns the ShortName of the first resource type
// that has ExcludeFromIssueBadge set, or "" if none exists.
func firstExcludedShortName() string {
	for _, rt := range resource.AllResourceTypes() {
		if rt.ExcludeFromIssueBadge {
			return rt.ShortName
		}
	}
	return ""
}

// allTypes is a convenience alias used in scenarios that must cover every type.
func allTypeDefs() []resource.ResourceTypeDef {
	return resource.AllResourceTypes()
}

// ---------------------------------------------------------------------------
// Parity scenarios
// ---------------------------------------------------------------------------

// TestMenuRenderParity is the top-level table-driven parity gate.
// Each subtest builds the same logical state on both sides and asserts
// byte-identical output from View() and RenderBody().
func TestMenuRenderParity(t *testing.T) {
	// -----------------------------------------------------------------------
	// S1: Default state — no availability, no issues, no filter.
	// -----------------------------------------------------------------------
	t.Run("S1_Default", func(t *testing.T) {
		m, c := newParityPair(t, 80, 200)
		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S2: Availability counts — positive, confirmed-empty (dimmed),
	//     truncated-zero (not dimmed, "(0+)"), truncated-positive ("(5+)").
	// -----------------------------------------------------------------------
	t.Run("S2_Availability", func(t *testing.T) {
		all := allTypeDefs()
		if len(all) < 4 {
			t.Skip("need at least 4 resource types for S2")
		}
		typePos := all[0].ShortName    // positive count
		typeEmpty := all[1].ShortName  // confirmed-empty (known=0, not truncated)
		typeTrunc0 := all[2].ShortName // truncated-zero (known=0, truncated=true)
		typeTruncPos := all[3].ShortName // truncated-positive (known=5, truncated=true)

		m, c := newParityPair(t, 80, 200)

		// Legacy side
		m.SetAvailability(typePos, 7)
		m.SetTruncated(typePos, false)
		m.SetAvailability(typeEmpty, 0)
		m.SetTruncated(typeEmpty, false)
		m.SetAvailability(typeTrunc0, 0)
		m.SetTruncated(typeTrunc0, true)
		m.SetAvailability(typeTruncPos, 5)
		m.SetTruncated(typeTruncPos, true)

		// Controller side — PatchMenuAvailability for each
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{ResourceType: typePos, Count: 7, Truncated: false},
			runtime.PatchMenuAvailability{ResourceType: typeEmpty, Count: 0, Truncated: false},
			runtime.PatchMenuAvailability{ResourceType: typeTrunc0, Count: 0, Truncated: true},
			runtime.PatchMenuAvailability{ResourceType: typeTruncPos, Count: 5, Truncated: true},
		})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S3: Unknown availability — type not in any availability map.
	//     Rendered normal, no count suffix.
	// -----------------------------------------------------------------------
	t.Run("S3_UnknownAvailability", func(t *testing.T) {
		all := allTypeDefs()
		if len(all) < 2 {
			t.Skip("need at least 2 resource types for S3")
		}
		knownType := all[0].ShortName
		// unknownType is all[1] — intentionally left out of both maps.

		m, c := newParityPair(t, 80, 200)

		// Set availability only for knownType; leave all[1] absent.
		m.SetAvailability(knownType, 3)
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{ResourceType: knownType, Count: 3, Truncated: false},
		})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S4: Issue badges — non-excluded type with count>0 gets " issues:N";
	//     known type with count==0 gets no badge.
	// -----------------------------------------------------------------------
	t.Run("S4_IssueBadges", func(t *testing.T) {
		nonExcluded := firstNonExcludedShortName(t)
		all := allTypeDefs()
		// Find a second non-excluded type for the zero-issue case.
		var zeroIssueType string
		for _, rt := range all {
			if rt.ShortName != nonExcluded && !rt.ExcludeFromIssueBadge {
				zeroIssueType = rt.ShortName
				break
			}
		}
		if zeroIssueType == "" {
			t.Skip("need at least 2 non-excluded resource types for S4")
		}

		m, c := newParityPair(t, 80, 200)

		// Legacy side
		m.SetIssues(nonExcluded, 3, false)
		m.SetIssues(zeroIssueType, 0, false)

		// Controller side
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenu{ResourceType: nonExcluded, Issues: 3, Truncated: false},
			runtime.PatchMenu{ResourceType: zeroIssueType, Issues: 0, Truncated: false},
		})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S5: Filter active (≥2 chars) narrowing the list.
	// -----------------------------------------------------------------------
	t.Run("S5_FilterActive", func(t *testing.T) {
		m, c := newParityPair(t, 80, 200)

		// "ec" should match EC2-related types without being too narrow.
		m.SetFilter("ec")
		c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "ec"})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S5b: Filter with no matches.
	// -----------------------------------------------------------------------
	t.Run("S5b_FilterNoMatch", func(t *testing.T) {
		m, c := newParityPair(t, 80, 200)

		m.SetFilter("zzznomatch")
		c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "zzznomatch"})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S6: Attention-only (ctrl+z) toggled, post-probe.
	// -----------------------------------------------------------------------
	t.Run("S6_AttentionOnly", func(t *testing.T) {
		nonExcluded := firstNonExcludedShortName(t)
		all := allTypeDefs()
		var secondNonExcluded string
		for _, rt := range all {
			if rt.ShortName != nonExcluded && !rt.ExcludeFromIssueBadge {
				secondNonExcluded = rt.ShortName
				break
			}
		}

		m, c := newParityPair(t, 80, 200)

		// Load issues first so attention filter has something to show.
		m.SetIssues(nonExcluded, 2, false)
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenu{ResourceType: nonExcluded, Issues: 2, Truncated: false},
		})

		// If we have a second type, mark it as known-zero so it's hidden.
		if secondNonExcluded != "" {
			m.SetIssues(secondNonExcluded, 0, false)
			c.ApplyIntents([]runtime.UIIntent{
				runtime.PatchMenu{ResourceType: secondNonExcluded, Issues: 0, Truncated: false},
			})
		}

		// Toggle attention filter on both sides.
		m.Toggle()
		m.SetFilter("") // re-apply filter after toggle
		c.Apply(app.Action{Kind: app.ActionToggleAttention})

		// Re-apply on model side (Toggle doesn't call applyFilter directly for
		// the exported Toggle method; SetFilter("") re-triggers it).
		// Actually SetFilter with empty text still calls applyFilter, so we need
		// to call it more carefully. The model's Toggle is called in Update for
		// ToggleAttentionOnly key, which calls applyFilter internally. We call
		// SetFilter to reset to trigger re-filter after Toggle.
		// Clear filter then re-set to trigger applyFilter consistently.
		m.SetFilter("")

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S7a: Narrow width (40) forcing name truncation.
	// -----------------------------------------------------------------------
	t.Run("S7a_NarrowWidth40", func(t *testing.T) {
		m, c := newParityPair(t, 40, 200)
		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S7b: Wide width (200).
	// -----------------------------------------------------------------------
	t.Run("S7b_WideWidth200", func(t *testing.T) {
		m, c := newParityPair(t, 200, 200)
		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S8: Scrolled — enough MoveDown to push scrollOffset > 0.
	// -----------------------------------------------------------------------
	t.Run("S8_Scrolled", func(t *testing.T) {
		// Small viewport so scrolling kicks in quickly.
		m, c := newParityPair(t, 80, 5)

		// Move down enough to scroll — 8 steps should push past the first category.
		for i := 0; i < 8; i++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
			c.Apply(app.Action{Kind: app.ActionMoveDown})
		}

		body := *c.Snapshot().Body.Menu
		// scrollOffset is managed by the model's adjustScroll; body.Selected is
		// set by the controller. We pass body (which has the controller's Selected)
		// to RenderBody which uses the model's scrollOffset — so both must agree
		// on what is visible. They will if the cursor positions match.
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S9a: Selection on first item (default).
	// -----------------------------------------------------------------------
	t.Run("S9a_SelectFirst", func(t *testing.T) {
		m, c := newParityPair(t, 80, 200)
		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S9b: Selection on middle item.
	// -----------------------------------------------------------------------
	t.Run("S9b_SelectMiddle", func(t *testing.T) {
		all := allTypeDefs()
		mid := len(all) / 2
		if mid == 0 {
			t.Skip("not enough resource types for middle selection test")
		}

		m, c := newParityPair(t, 80, 200)

		for i := 0; i < mid; i++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
			c.Apply(app.Action{Kind: app.ActionMoveDown})
		}

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S9c: Selection on last item.
	// -----------------------------------------------------------------------
	t.Run("S9c_SelectLast", func(t *testing.T) {
		all := allTypeDefs()
		last := len(all) - 1
		if last <= 0 {
			t.Skip("not enough resource types for last selection test")
		}

		m, c := newParityPair(t, 80, 200)

		for i := 0; i < last; i++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
			c.Apply(app.Action{Kind: app.ActionMoveDown})
		}

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S10: SetIssuesFromCache bulk-load — same data path as cache restore.
	// -----------------------------------------------------------------------
	t.Run("S10_IssuesFromCache", func(t *testing.T) {
		nonExcluded := firstNonExcludedShortName(t)
		all := allTypeDefs()
		var second string
		for _, rt := range all {
			if rt.ShortName != nonExcluded && !rt.ExcludeFromIssueBadge {
				second = rt.ShortName
				break
			}
		}

		m, c := newParityPair(t, 80, 200)

		counts := map[string]int{nonExcluded: 5}
		trunc := map[string]bool{nonExcluded: false}
		known := map[string]bool{nonExcluded: true}
		if second != "" {
			counts[second] = 0
			trunc[second] = false
			known[second] = true
		}

		// Legacy side: bulk-load from cache.
		m.SetIssuesFromCache(counts, trunc, known)

		// Controller side: PatchMenuIssueBatch mirrors SetIssuesFromCache.
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuIssueBatch{
				Counts:   counts,
				Truncated: trunc,
				Known:    known,
			},
		})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})

	// -----------------------------------------------------------------------
	// S11: Availability + issue badges together (combined state).
	// -----------------------------------------------------------------------
	t.Run("S11_AvailabilityPlusIssues", func(t *testing.T) {
		all := allTypeDefs()
		if len(all) < 2 {
			t.Skip("need at least 2 resource types for S11")
		}
		typeA := all[0].ShortName
		var typeB string
		for _, rt := range all {
			if rt.ShortName != typeA && !rt.ExcludeFromIssueBadge {
				typeB = rt.ShortName
				break
			}
		}
		if typeB == "" {
			t.Skip("need a second non-excluded type for S11")
		}

		m, c := newParityPair(t, 80, 200)

		// Availability
		m.SetAvailability(typeA, 10)
		m.SetTruncated(typeA, false)
		// Issues on same type
		m.SetIssues(typeA, 4, false)
		// Zero issues on second type
		m.SetIssues(typeB, 0, false)

		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{ResourceType: typeA, Count: 10, Truncated: false},
			runtime.PatchMenu{ResourceType: typeA, Issues: 4, Truncated: false},
			runtime.PatchMenu{ResourceType: typeB, Issues: 0, Truncated: false},
		})

		body := *c.Snapshot().Body.Menu
		assertParity(t, &m, body)
	})
}
