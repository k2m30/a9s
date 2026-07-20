//go:build integration

package integration

// scenario_all_types_reference_test.go — the all-types generalization of the
// s3 reference walk (scenario_s3_reference_test.go): every registered
// top-level resource type runs the same per-surface gate in demo mode, with
// no per-type special cases. Surfaces per type: main-menu count, list open
// (frame count + first row render), detail fields body, related-panel
// witness among sampled rows, YAML and JSON id visibility, back-navigation
// stack integrity, and a full fetch-origin cache entry. Related drills stay
// with related_view_validation_test.go and
// scenario_related_drill_through_test.go; deep render assertions stay with
// the per-type visual gates.

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// allTypesSampledRows bounds the related-witness walk. The pivot-coverage
// gate (tests/unit/qa_demo_pivot_coverage_test.go) guarantees a witness
// exists on SOME fixture row, but not necessarily the first; sampling keeps
// the per-type wall cost small while still finding it in practice.
const allTypesSampledRows = 5

func TestScenario_AllTypesReferenceSurfaces(t *testing.T) {
	counts := demofixtures.ExpectedTopLevelCountsForTest()

	// One startup scenario renders the availability-prefetched main menu for
	// every type at once; per-type walk scenarios skip the startup chain to
	// keep the 60+-type sweep cheap.
	menuScenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, menuScenario)
	menuView := menuScenario.currentView()

	childShortNames := resource.AllChildShortNamesForTest()
	childSet := make(map[string]bool, len(childShortNames))
	for _, name := range childShortNames {
		childSet[name] = true
	}

	for _, shortName := range resource.AllShortNames() {
		shortName := shortName

		// Same skip logic as related_view_validation_test.go: child types
		// have no top-level list, ct-events is the self-referencing event
		// stream.
		if shortName == "ct-events" || childSet[shortName] {
			continue
		}

		t.Run(shortName, func(t *testing.T) {
			rt := resource.FindResourceType(shortName)
			if rt == nil {
				t.Fatalf("resource type %q not found in registry", shortName)
			}
			count, ok := counts[shortName]
			if !ok {
				t.Fatalf("no ExpectedTopLevelCounts entry for %q — every top-level type needs a demo count oracle (core/demo/fixtures/counts.go)", shortName)
			}

			t.Run("menu", func(t *testing.T) {
				// Same "Name (count)" render fullIntegrationAssertMainMenuCounts
				// keys on; for count==0 this asserts the dim "(0)" row.
				want := fmt.Sprintf("%s (%d)", rt.Name, count)
				if !strings.Contains(menuView, want) {
					t.Fatalf("main menu missing %q for type %s\nmenu view:\n%s", want, shortName, menuView)
				}
			})

			if count == 0 {
				t.Skipf("%s: no demo fixtures; menu dim state (0) asserted", shortName)
			}

			sc := fullIntegrationNewDemoScenario(t)
			var (
				rows            []resource.Resource
				first           resource.Resource
				relatedSnapshot map[string]messages.RelatedCheckResult
			)

			frameTitle := fullIntegrationFrameCount(shortName, fullIntegrationCountExpectation{count: count})

			t.Run("list", func(t *testing.T) {
				sc.Command(shortName)
				sc.ExpectNoAPIError()
				sc.ExpectCurrentListType(shortName)
				sc.ExpectFrameContains(frameTitle)
				sc.ExpectLoadedCount(count)
				rows = append([]resource.Resource(nil), sc.currentListResources...)
				first = rows[0]
				allTypesExpectRowRendered(sc, shortName, first)
			})

			t.Run("detail", func(t *testing.T) {
				if len(rows) == 0 {
					t.Skip("list surface did not load rows")
				}
				sc.OpenDetailResource(shortName, first)
				sc.ExpectNoAPIError()
				sc.ExpectCurrentResourceID(first.ID)
				if sc.currentResource == nil || len(sc.currentResource.Fields) == 0 {
					sc.failf("detail for %s %q rendered with an empty fields body", shortName, first.ID)
				}
				// Snapshot immediately after the first open: re-entering this
				// detail later hits relatedCache and never re-emits
				// RelatedCheckResultMsg (same pattern as
				// related_view_validation_test.go).
				relatedSnapshot = maps.Clone(sc.lastRelatedByName)
			})

			t.Run("related", func(t *testing.T) {
				if len(rows) == 0 {
					t.Skip("list surface did not load rows")
				}
				defs := resource.GetRelated(shortName)
				if len(defs) == 0 {
					sc.Back()
					t.Skipf("%s: no registered pivots", shortName)
				}

				sampled := min(allTypesSampledRows, len(rows))
				sawCountable := false
				for i := 0; i < sampled; i++ {
					snap := relatedSnapshot
					if i > 0 {
						sc.OpenDetailResource(shortName, rows[i])
						snap = maps.Clone(sc.lastRelatedByName)
					}

					witness := ""
					for _, def := range defs {
						res, ok := snap[def.DisplayName]
						if !ok {
							continue
						}
						if res.Result.State == domain.RelatedResolved {
							sawCountable = true
						}
						if witness == "" && res.Result.Count > 0 {
							witness = def.DisplayName
						}
					}
					if witness == "" {
						// State: RelatedDeferred (server-side FetchFilter pivot) is a
						// navigable pivot — the same actionable rule
						// related_view_validation_test.go applies.
						for _, def := range defs {
							res, ok := snap[def.DisplayName]
							if ok && res.Result.State == domain.RelatedDeferred {
								witness = def.DisplayName
								break
							}
						}
					}
					if witness != "" {
						sc.ExpectRelatedRow(witness)
						sc.Back()
						return
					}
					sc.Back()
				}

				if !sawCountable {
					t.Skipf("%s: every pivot returned a non-Resolved state (Unknown/Deferred/Error) with no FetchFilter witness across %d sampled row(s) — budget-excluded-only type (cf. knownDisconnectedPivots in tests/unit/qa_demo_pivot_coverage_test.go)", shortName, sampled)
				}
				t.Errorf("%s: countable pivots exist but no witness (count>0 or fetch-filter pivot) among %d of %d fixture row(s) — fixture graph gap (core/demo/fixtures/, contract docs/related-resources.md)", shortName, sampled, len(rows))
			})

			t.Run("yaml", func(t *testing.T) {
				if len(rows) == 0 {
					t.Skip("list surface did not load rows")
				}
				sc.OpenDetailResource(shortName, first)
				sc.OpenYAML()
				allTypesExpectIDVisible(sc, first.ID, "yaml")
				sc.Back()
			})

			t.Run("json", func(t *testing.T) {
				if len(rows) == 0 {
					t.Skip("list surface did not load rows")
				}
				sc.Press("J")
				sc.ExpectFrameContains("json")
				allTypesExpectIDVisible(sc, first.ID, "json")
				sc.Back()
			})

			t.Run("back_navigation", func(t *testing.T) {
				if len(rows) == 0 {
					t.Skip("list surface did not load rows")
				}
				sc.Back()
				sc.ExpectFrameContains(frameTitle)
				sc.Back()
				// The list visit synced the exact total to the menu row
				// (core/app/handle.go syncExactTotalToMenu).
				sc.ExpectViewContains(fmt.Sprintf("%s (%d)", rt.Name, count))
			})

			t.Run("cache", func(t *testing.T) {
				entry, ok := sc.model.Core().ResourceCache(shortName)
				if !ok || entry == nil {
					t.Fatalf("no full fetch-origin cache entry for %s after list load (core/runtime/accessors.go ResourceCache)", shortName)
				}
				if got := len(entry.Resources); got != count {
					t.Fatalf("%s cache entry rows = %d, expected %d per core/demo/fixtures/counts.go ExpectedTopLevelCounts", shortName, got, count)
				}
			})
		})
	}
}

// allTypesExpectRowRendered asserts the list view renders the first fixture
// row. Identity columns may show a different representation of the resource
// than res.ID (e.g. the ecs-task Task ID column renders the full ARN,
// ellipsis-truncated before the bare-id suffix), so any rendered field value
// of the row also counts as evidence.
func allTypesExpectRowRendered(s *fullIntegrationScenario, shortName string, res resource.Resource) {
	s.t.Helper()
	if s.findRow(res.ID) != "" {
		return
	}
	view := s.currentView()
	if res.Name != "" && strings.Contains(view, res.Name) {
		return
	}
	for _, val := range res.Fields {
		if len(val) >= 3 && strings.Contains(view, val) {
			return
		}
	}
	s.failf("first %s row %q (name %q) not rendered in list view", shortName, res.ID, res.Name)
}

// allTypesExpectIDVisible asserts the resource ID appears in the current
// YAML/JSON view. The compacted fallback tolerates viewport line wrap and
// right-edge padding splitting a long ID across rendered lines.
func allTypesExpectIDVisible(s *fullIntegrationScenario, id, surface string) {
	s.t.Helper()
	view := s.currentView()
	if strings.Contains(view, id) {
		return
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(view, "\n", ""), " ", "")
	if strings.Contains(compact, id) {
		return
	}
	s.failf("%s view does not contain resource id %q", surface, id)
}
