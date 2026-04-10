//go:build integration

package integration

import (
	"context"
	"maps"
	"os"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestFullRelatedViewValidation is a comprehensive integration test that validates
// ALL related-resource behavior across ALL resource types — both right-column related
// entries and left-column navigable fields. It operates in demo mode by default
// (when A9S_CT_PROFILE is not set) and in live mode when A9S_CT_PROFILE is set.
//
// Algorithm:
//   - For each resource type (skip ct-events, skip child types):
//   - Try to find any resource; skip the type if none available
//   - Open detail view
//   - Right column: snapshot lastRelatedByName once, call checker independently,
//     compare counts; for actionable defs, follow in a fresh scenario
//   - Left column: verify TargetType resolves for each navigable field
//   - Back to list
func TestFullRelatedViewValidation(t *testing.T) {
	profile := os.Getenv("A9S_CT_PROFILE")
	region := os.Getenv("A9S_CT_REGION")

	var rootScenario *fullIntegrationScenario
	if profile != "" {
		rootScenario = fullIntegrationNewLiveScenario(t, profile, region)
	} else {
		rootScenario = fullIntegrationNewDemoScenario(t)
	}

	// Build a skip set for child types — they have no top-level list to open.
	childShortNames := resource.AllChildShortNames()
	childSet := make(map[string]bool, len(childShortNames))
	for _, name := range childShortNames {
		childSet[name] = true
	}

	for _, shortName := range resource.AllShortNames() {
		shortName := shortName // capture for subtest

		// Skip self-referencing and child types.
		if shortName == "ct-events" {
			continue
		}
		if childSet[shortName] {
			continue
		}

		t.Run(shortName, func(t *testing.T) {
			rt := resource.FindResourceType(shortName)
			if rt == nil {
				t.Skipf("resource type %q not found in registry", shortName)
			}

			pf := resource.GetPaginatedFetcher(shortName)
			if pf == nil {
				t.Skipf("resource type %q has no paginated fetcher", shortName)
			}

			ctx := context.Background()
			result, err := pf(ctx, rootScenario.clients, "")
			if err != nil {
				t.Skipf("fetcher for %q failed (may be unavailable in this environment): %v", shortName, err)
			}
			if len(result.Resources) == 0 {
				t.Skipf("no %q resources available", shortName)
			}

			// Test up to 3 resources per type to avoid combinatorial explosion.
			limit := 3
			if len(result.Resources) < limit {
				limit = len(result.Resources)
			}

			for idx := 0; idx < limit; idx++ {
				res := result.Resources[idx]
				t.Run(res.ID, func(t *testing.T) {
					// Each resource gets its own scenario to isolate state.
					var sc *fullIntegrationScenario
					if profile != "" {
						sc = fullIntegrationNewLiveScenario(t, profile, region)
					} else {
						sc = fullIntegrationNewDemoScenario(t)
					}

					sc.OpenDetailResource(shortName, res)

					// Snapshot lastRelatedByName once, immediately after opening detail.
					// Subsequent FollowRelated calls in subtests use a fresh scenario each
					// time, avoiding the relatedCache re-entry problem where a second
					// OpenDetailResource hits the cache and never re-emits RelatedCheckResultMsg.
					relatedSnapshot := maps.Clone(sc.lastRelatedByName)

					// RIGHT COLUMN: validate related entries.
					defs := resource.GetRelated(shortName)
					for _, def := range defs {
						def := def // capture
						t.Run("related/"+def.DisplayName, func(t *testing.T) {
							// TODO: This is currently tautological — the checker is the same code path
							// the UI uses. A truly independent check would call the underlying AWS API
							// directly (e.g., ec2:DescribeVolumes to verify EBS count) and compare
							// against what the UI displays. For now, calling the checker directly with
							// an empty ResourceCache verifies that the checker runs without panic and
							// returns a structurally valid result.
							//
							// Pass an empty ResourceCache{}. NeedsTargetCache checkers will fetch
							// the target type from the demo/live clients and return the real count;
							// field-only checkers return their count from the resource fields alone.
							checker := def.Checker
							if checker == nil {
								t.Skipf("def %q has nil checker", def.DisplayName)
							}
							expected := checker(ctx, sc.clients, res, resource.ResourceCache{})

							// Use the snapshot taken immediately after OpenDetailResource — before
							// any FollowRelated calls could reset lastRelatedByName via cache re-entry.
							uiMsg, ok := relatedSnapshot[def.DisplayName]
							if !ok {
								// The UI did not emit a result for this def. This is unexpected
								// if the checker returned a definitive count (not -1).
								if expected.Count != -1 {
									t.Errorf("related %q: checker returned Count=%d but detail view never emitted a RelatedCheckResultMsg",
										def.DisplayName, expected.Count)
								}
								return
							}

							// Only compare counts when the checker returned a definitive result.
							// When checker=-1 (NeedsTargetCache=true with empty cache), the checker
							// could not compute the count — skip the comparison in that case, but
							// still verify navigation below.
							if expected.Count != -1 && uiMsg.Result.Count != expected.Count {
								t.Errorf("related %q: count mismatch: UI=%d, checker=%d",
									def.DisplayName, uiMsg.Result.Count, expected.Count)
							}

							// If actionable (count > 0), follow the related entry and verify navigation.
							// Use a fresh scenario to avoid polluting the main scenario's state.
							if uiMsg.Result.Count > 0 || (uiMsg.Result.Count == -1 && len(uiMsg.Result.FetchFilter) > 0) {
								// Fresh scenario for navigation — avoids relatedCache hit problem on re-entry.
								var navSc *fullIntegrationScenario
								if profile != "" {
									navSc = fullIntegrationNewLiveScenario(t, profile, region)
								} else {
									navSc = fullIntegrationNewDemoScenario(t)
								}
								navSc.OpenDetailResource(shortName, res)
								navSc.ExpectRelatedRow(def.DisplayName)
								navSc.FollowRelated(def.DisplayName)
								navSc.ExpectNoAPIError()
							}
						})
					}

					// LEFT COLUMN: validate navigable fields.
					navFields := resource.GetNavigableFields(shortName)
					for _, nf := range navFields {
						nf := nf // capture
						t.Run("nav/"+nf.FieldPath, func(t *testing.T) {
							// Verify that the target type resolves — catches mis-registered TargetType strings.
							_, _, found := resource.ResolveNavigationTarget(nf.TargetType)
							if !found {
								t.Errorf("NavigableField %q on %q: TargetType %q does not resolve to a known resource type",
									nf.FieldPath, shortName, nf.TargetType)
								return
							}

							// Check the field value. Falls back to Fields map.
							val := res.Fields[nf.FieldPath]
							if val == "" {
								// Field may be empty for this particular resource.
								t.Skipf("field %q is empty on resource %q — cannot test navigation", nf.FieldPath, res.ID)
								return
							}

							// TODO: Add NavigateField(fieldPath string) to the harness so we can
							// exercise actual cursor-to-field-and-Enter navigation here. For now we
							// only verify that the TargetType resolves (above) and the field value
							// is non-empty — this catches registration bugs without requiring
							// harness changes.
							_ = val
						})
					}
				})
			}
		})
	}
}

