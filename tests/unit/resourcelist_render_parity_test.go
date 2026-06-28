// resourcelist_render_parity_test.go — byte-parity gate for PR-C list flip.
//
// Asserts that ResourceListModel.RenderList(body) produces output byte-identical
// to the legacy ResourceListModel.View() for the same logical list state,
// across EVERY resource type in resource.AllResourceTypes() and a set of
// scenarios per type.
//
// Known gap documented here: resolveIdentityColumn step 3 (column Path contains
// "Name"/"Identifier") is not reproducible from ColumnDef (which has no Path
// field), so the controller's resolveListMarkerCol falls back to column 0 for
// such types. Scenario S13_EnrichmentFindings is the primary detector: it
// renders enrichment glyphs and will misplace the marker for any type where
// step 3 would fire in View() but resolveListMarkerCol chose a different index.
//
// Strategy:
//   - Legacy side: ResourceListModel constructed via NewResourceList (loading) or
//     NewResourceListFromCache (pre-loaded, for filter/sort scenarios), sized with
//     SetSize, then SetEnrichmentState for enrichment scenarios. View() is called.
//   - Controller side: push ScreenResourceList via ActionCommand, seed resources
//     via ApplyResourcesLoaded (test-only export from internal/app/export_test.go),
//     drive state via Apply(Action) to match. Snapshot().Body.List provides body.
//   - Call m.RenderList(body) on the SAME sized model m.
//   - Assert got == legacy EXACTLY. On mismatch: t.Errorf with type, scenario,
//     and a full line-by-line diff. Do NOT suppress or normalise.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Parity assertion
// ---------------------------------------------------------------------------

// assertListParity asserts byte-identical output from View() and RenderList(body).
// On mismatch it emits a full line-by-line diff with type + scenario context.
func assertListParity(t *testing.T, typeName, scenario string, m *views.ResourceListModel, body app.ListBody) {
	t.Helper()
	legacy := m.View()
	got := m.RenderList(body)
	if got == legacy {
		return
	}
	legacyLines := strings.Split(legacy, "\n")
	gotLines := strings.Split(got, "\n")
	maxLines := len(legacyLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf(
		"[%s / %s] RenderList differs from View() — View() %d lines, RenderList %d lines\n",
		typeName, scenario, len(legacyLines), len(gotLines),
	))
	for i := range maxLines {
		leg, got2 := "", ""
		if i < len(legacyLines) {
			leg = legacyLines[i]
		}
		if i < len(gotLines) {
			got2 = gotLines[i]
		}
		if leg != got2 {
			diff.WriteString(fmt.Sprintf(
				"  line %d:\n    View():     %q\n    RenderList: %q\n",
				i+1, leg, got2,
			))
		}
	}
	t.Errorf("byte-parity FAILED:\n%s", diff.String())
}

// ---------------------------------------------------------------------------
// Controller setup helper
// ---------------------------------------------------------------------------

// newListController builds a Controller pre-navigated to a ScreenResourceList
// for the given resource type ShortName. Uses NO_COLOR + temp config dir so
// output is deterministic.
func newListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	// Navigate to the list screen for this resource type.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// ---------------------------------------------------------------------------
// Synthetic resource builders
// ---------------------------------------------------------------------------

// listParityResources builds n deterministic resources for any ResourceTypeDef.
// Fields are populated from the type's declared Columns so lifecycle-widening
// fires on status columns. Produces varied lifecycle values so some rows differ.
func listParityResources(td resource.ResourceTypeDef, n int) []resource.Resource {
	statuses := []string{"running", "stopped", "pending", "available", "active", "terminated"}
	lk := td.LifecycleKey
	if lk == "" {
		lk = "state"
	}
	results := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("%s-%03d", td.ShortName, i+1)
		fields := make(map[string]string, len(td.Columns)+4)

		// Populate every declared column key with deterministic values.
		for _, col := range td.Columns {
			switch {
			case col.Key == "name" || strings.Contains(strings.ToLower(col.Key), "name"):
				fields[col.Key] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
			case col.Key == lk || col.Key == "state" || col.Key == "status":
				fields[col.Key] = statuses[i%len(statuses)]
			default:
				fields[col.Key] = fmt.Sprintf("v-%s-%d", col.Key, i+1)
			}
		}

		// Always ensure "name" and lifecycle key are populated for resolver cascade.
		fields["name"] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
		fields[lk] = statuses[i%len(statuses)]

		results[i] = resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("demo-%s-%d", td.ShortName, i+1),
			Fields: fields,
		}
	}
	return results
}

// listParityFindings builds a finding map for roughly half the given resources,
// alternating between SevBroken and SevWarn to exercise both glyph paths.
func listParityFindings(resources []resource.Resource) map[string]domain.Finding {
	if len(resources) == 0 {
		return nil
	}
	out := make(map[string]domain.Finding, len(resources)/2+1)
	for i, r := range resources {
		if i%2 == 0 {
			sev := domain.SevBroken
			if i%4 == 0 {
				sev = domain.SevWarn
			}
			out[r.ID] = domain.Finding{
				Code:     "PARITY-TEST",
				Phrase:   "test finding",
				Severity: sev,
			}
		}
	}
	return out
}

// firstSortableColKey returns the first column key that is non-empty and not
// a lifecycle/status key, suitable for sort toggle testing.
func firstSortableColKey(td resource.ResourceTypeDef) string {
	for _, c := range td.Columns {
		if c.Key != "" {
			return c.Key
		}
	}
	return "name"
}

// ---------------------------------------------------------------------------
// Main parity test — ALL types × ALL scenarios
// ---------------------------------------------------------------------------

// TestResourceListRenderParity is the byte-parity gate for the PR-C list flip.
// Each subtest name is "TypeShortName/ScenarioName".
//
// Mismatch = real regression in RenderList. Report it; do NOT loosen the
// assertion. The architect decides how to fix RenderList (e.g. add Path to
// ColumnDef) or accepts the delta.
func TestResourceListRenderParity(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	const (
		stdW = 160
		stdH = 30
	)

	for _, td := range allTypes {
		td := td // capture loop variable
		t.Run(td.ShortName, func(t *testing.T) {
			// Ensure NO_COLOR for deterministic styled output.
			t.Setenv("NO_COLOR", "1")
			styles.Reinit()
			t.Cleanup(styles.Reinit)

			k := keys.Default()
			resources10 := listParityResources(td, 10)
			findings := listParityFindings(resources10)
			lk := td.LifecycleKey
			if lk == "" {
				lk = "state" //nolint:ineffassign // retained for future sort-by-lifecycle scenarios
			}
			_ = lk

			// ── S1: Loading state (no resources loaded) ───────────────────────
			// Both sides: model is in loading=true, body.Loading=true.
			t.Run("S1_Loading", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S1_Loading", &m, body)
			})

			// ── S2: Default (loaded, no filter/sort/attention) ────────────────
			t.Run("S2_Default", func(t *testing.T) {
				// Legacy: update with ResourcesLoaded.
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S2_Default", &m, body)
			})

			// ── S3: Empty list (zero resources) ───────────────────────────────
			t.Run("S3_Empty", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    []resource.Resource{},
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, []resource.Resource{}, nil, false)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S3_Empty", &m, body)
			})

			// ── S4: Single resource ───────────────────────────────────────────
			t.Run("S4_Single", func(t *testing.T) {
				one := listParityResources(td, 1)

				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    one,
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, one, nil, false)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S4_Single", &m, body)
			})

			// ── S5: Filter matching subset ────────────────────────────────────
			// Legacy: NewResourceListFromCache with filterText pre-set.
			// Controller: ApplyResourcesLoaded then ActionSetFilter.
			t.Run("S5_Filter", func(t *testing.T) {
				// Filter on "-00" matches rows 001..009 but not 010.
				filterArg := fmt.Sprintf("demo-%s-00", td.ShortName)

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					filterArg,
					views.SortColNone, true, // sortColIdx=SortColNone, sortAsc=true
					0, 0, false,             // cursorPos, hScrollOffset, attentionOnly
				)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: filterArg})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S5_Filter", &m, body)
			})

			// ── S6: Filter — no match ─────────────────────────────────────────
			t.Run("S6_FilterNoMatch", func(t *testing.T) {
				filterArg := "zzznomatch"

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					filterArg,
					views.SortColNone, true,
					0, 0, false,
				)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: filterArg})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S6_FilterNoMatch", &m, body)
			})

			// ── S7: Sort ascending by first column key ────────────────────────
			t.Run("S7_SortAsc", func(t *testing.T) {
				colKey := firstSortableColKey(td)

				// Find column index for NewResourceListFromCache.
				sortIdx := 0
				for i, c := range td.Columns {
					if c.Key == colKey {
						sortIdx = i
						break
					}
				}

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					"",
					sortIdx, true, // asc
					0, 0, false,
				)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S7_SortAsc", &m, body)
			})

			// ── S8: Sort descending ───────────────────────────────────────────
			t.Run("S8_SortDesc", func(t *testing.T) {
				colKey := firstSortableColKey(td)

				sortIdx := 0
				for i, c := range td.Columns {
					if c.Key == colKey {
						sortIdx = i
						break
					}
				}

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					"",
					sortIdx, false, // desc
					0, 0, false,
				)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				// Two ActionSort on same col: first → asc, second → desc.
				c.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
				c.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S8_SortDesc", &m, body)
			})

			// ── S9: Selection on first row (default, cursor=0) ────────────────
			t.Run("S9_SelectFirst", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S9_SelectFirst", &m, body)
			})

			// ── S10: Selection on middle row (cursor=5) ───────────────────────
			t.Run("S10_SelectMiddle", func(t *testing.T) {
				const mid = 5

				// Legacy: use Update with MoveDown key messages.
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})
				for range mid {
					m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
				}

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				for range mid {
					c.Apply(app.Action{Kind: app.ActionMoveDown})
				}
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S10_SelectMiddle", &m, body)
			})

			// ── S11: Selection on last row ────────────────────────────────────
			t.Run("S11_SelectLast", func(t *testing.T) {
				last := len(resources10) - 1

				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})
				for range last {
					m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
				}

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				for range last {
					c.Apply(app.Action{Kind: app.ActionMoveDown})
				}
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S11_SelectLast", &m, body)
			})

			// ── S12: Horizontal scroll (hScrollOffset=1) ─────────────────────
			t.Run("S12_HScrollOffset1", func(t *testing.T) {
				if len(td.Columns) < 2 {
					t.Skip("type has fewer than 2 columns — hscroll not applicable")
				}

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					"",
					views.SortColNone, true,
					0, 1, false, // hScrollOffset=1
				)
				m.SetSize(stdW, stdH)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.Apply(app.Action{Kind: app.ActionScrollRight})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S12_HScrollOffset1", &m, body)
			})

			// ── S13: Enrichment findings present, attention filter OFF ─────────
			// PRIMARY MarkerCol gap detector: View() uses resolveIdentityColumn with
			// full path cascade (steps 1-5); RenderList uses body.MarkerCol from
			// resolveListMarkerCol which cannot check column Path (no Path in
			// ColumnDef). Any type where step 3 would fire in View() but
			// resolveListMarkerCol chose a different index will fail here.
			t.Run("S13_EnrichmentFindings", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})
				m.SetEnrichmentState(len(findings), false, findings)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S13_EnrichmentFindings", &m, body)
			})

			// ── S14: Attention-only (ctrl+z) active with findings ─────────────
			t.Run("S14_AttentionOnly", func(t *testing.T) {
				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					"",
					views.SortColNone, true,
					0, 0, true, // attentionOnly=true
				)
				m.SetSize(stdW, stdH)
				m.SetEnrichmentState(len(findings), false, findings)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings)
				c.Apply(app.Action{Kind: app.ActionToggleAttention})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S14_AttentionOnly", &m, body)
			})

			// ── S15: Enrichment findings + horizontal scroll ───────────────────
			// Verifies that the marker column index survives hscroll on both sides.
			// The identity column may be scrolled off; both sides must agree on -1.
			t.Run("S15_FindingsWithHScroll", func(t *testing.T) {
				if len(td.Columns) < 2 {
					t.Skip("type has fewer than 2 columns — hscroll not applicable")
				}

				m := views.NewResourceListFromCache(
					td, nil, k,
					resources10, nil,
					"",
					views.SortColNone, true,
					0, 1, false, // hScrollOffset=1
				)
				m.SetSize(stdW, stdH)
				m.SetEnrichmentState(len(findings), false, findings)

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				c.ApplyEnrichmentState(td.ShortName, len(findings), false, findings)
				c.Apply(app.Action{Kind: app.ActionScrollRight})
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S15_FindingsWithHScroll", &m, body)
			})

			// ── S16: Narrow width (80) ────────────────────────────────────────
			t.Run("S16_NarrowWidth80", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(80, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				body := *c.Snapshot().Body.List

				// Set width on model after loading so RenderList and View() agree.
				assertListParity(t, td.ShortName, "S16_NarrowWidth80", &m, body)
			})

			// ── S17: Very narrow width (40) ────────────────────────────────────
			// May hit "No resources found" on some types when all columns are too
			// wide to fit. Both sides must agree on that path.
			t.Run("S17_VeryNarrowWidth40", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(40, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S17_VeryNarrowWidth40", &m, body)
			})

			// ── S18: Short height with scrolled cursor ─────────────────────────
			t.Run("S18_ShortHeightScrolled", func(t *testing.T) {
				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, 5)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    resources10,
					ResourceType: td.ShortName,
				})
				// Move cursor to row 7 so the visible window scrolls.
				for range 7 {
					m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
				}

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, resources10, nil, false)
				for range 7 {
					c.Apply(app.Action{Kind: app.ActionMoveDown})
				}
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S18_ShortHeightScrolled", &m, body)
			})

			// ── S19: Large list (50 rows), cursor at middle ────────────────────
			t.Run("S19_LargeList50Middle", func(t *testing.T) {
				large := listParityResources(td, 50)

				m := views.NewResourceList(td, nil, k)
				m.SetSize(stdW, stdH)
				m, _ = m.Update(messages.ResourcesLoaded{
					Resources:    large,
					ResourceType: td.ShortName,
				})
				for range 25 {
					m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
				}

				c := newListController(t, td.ShortName)
				c.ApplyResourcesLoaded(td.ShortName, large, nil, false)
				for range 25 {
					c.Apply(app.Action{Kind: app.ActionMoveDown})
				}
				body := *c.Snapshot().Body.List

				assertListParity(t, td.ShortName, "S19_LargeList50Middle", &m, body)
			})
		})
	}
}
