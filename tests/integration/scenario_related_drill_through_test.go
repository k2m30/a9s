//go:build integration

package integration

// scenario_related_drill_through_test.go — Drill-through regression pins.
//
// These tests verify that every registered related-panel pivot and navigable
// field on the graph-root fixture actually resolves to at least one real
// resource in the demo cache. An empty landing means the checker produced a
// resource ID in a format that does not match the target resource type's
// Resource.ID field — the exact bug class that caused the SES EventBridge and
// DDB KMS navigation failures.
//
// Test design:
//   - Uses DrillRelated to dispatch RelatedNavigateMsg for each pivot and
//     assert the resulting list is non-empty.
//   - Uses FollowNavigableField to dispatch RelatedNavigateMsg from a registered
//     NavigableField and assert the resulting resource lands.
//
// Table-driven design: drillThroughFixtures is the single source of truth for
// which (shortName, graphRoot) pairs are exercised. Adding a new resource type
// means adding one row here — no new test function.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// drillThroughFixtures is the full set of (label, shortName, graphRoot) triples
// that every drill-through subtest runs against. Multiple rows per shortName are
// allowed when a resource has more than one graph-root-equivalent fixture worth
// asserting (e.g. dbi carries both the baseline and Aurora fixtures).
var drillThroughFixtures = []struct {
	label     string
	shortName string
	graphRoot string
}{
	{"ses", "ses", demofixtures.SESGraphRootIdentity},
	{"ddb/orders-prod", "ddb", demofixtures.OrdersProdID},
	{"dbi/prod-dbi-1", "dbi", demofixtures.ProdDbiID},
	{"dbi/prod-dbi-aurora", "dbi", demofixtures.ProdDbiAuroraID},
	{"dbc/acme-docdb-prod", "dbc", demofixtures.ProdDbcID},
	{"dbc/prod-aurora", "dbc", "prod-aurora-cluster"},
	{"redis/prod-redis", "redis", demofixtures.ProdRedisID},
	{"s3/a9s-demo-healthy", "s3", demofixtures.HealthyBucketName},
	{"backup/plan-broken-2failed", "backup", demofixtures.ProdDatabasePlanID},
	// redshift: two graph-roots because logs (CloudWatch) and s3 (audit bucket)
	// are mutually exclusive per AWS LogDestinationType. Each root covers 10 of
	// 11 `count shown: yes` pivots; together they cover all 11.
	// See docs/resources/redshift-impl-plan.md §5.1.
	{"redshift/acme-warehouse", "redshift", demofixtures.AcmeWarehouseID},
	{"redshift/acme-reporting", "redshift", demofixtures.AcmeReportingID},
}

// ---------------------------------------------------------------------------
// TestScenario_RelatedDrillThrough_All
// ---------------------------------------------------------------------------

// TestScenario_RelatedDrillThrough_All verifies that every registered related-panel
// pivot with Count >= 1 on each graph-root fixture resolves to a non-empty resource
// list when DrillRelated is called.
//
// Catches ID-format mismatches where the checker emits a resource ID that does
// not match the target type's Resource.ID field (e.g., full ARN vs. bare name).
func TestScenario_RelatedDrillThrough_All(t *testing.T) {
	for _, tc := range drillThroughFixtures {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			scenario := fullIntegrationNewDemoScenario(t)
			runDemoStartup(t, scenario)

			scenario.OpenList(tc.shortName)
			root := fullIntegrationMustFindResourceByID(t, scenario.clients, tc.shortName, tc.graphRoot)
			scenario.OpenDetailResource(tc.shortName, root)
			scenario.ExpectNoAPIError()

			// Wait for every registered pivot to emit a RelatedCheckResultMsg.
			for _, def := range resource.GetRelated(tc.shortName) {
				scenario.ExpectRelatedRow(def.DisplayName)
			}

			// Drill every pivot with Count >= 1.
			for _, def := range resource.GetRelated(tc.shortName) {
				msg, ok := scenario.lastRelatedByName[def.DisplayName]
				if !ok || msg.Result.Count < 1 {
					t.Logf("[%s] %q: Count=%d — skipping drill", tc.label, def.DisplayName, msg.Result.Count)
					continue
				}
				landed := scenario.DrillRelated(def.DisplayName)
				if len(landed) == 0 {
					t.Errorf("[%s] DrillRelated(%q): landed empty; checker ResourceIDs=%v",
						tc.label, def.DisplayName, msg.Result.ResourceIDs)
					scenario.Press("esc")
					continue
				}
				// Generic ARN-format assertion: for target types with a registered
				// NavID extractor, returned IDs must be in bare form.
				assertBareIDs(t, tc.label, def.DisplayName, def.TargetType, landed)
				t.Logf("[%s] DrillRelated(%q): landed %d — %v",
					tc.label, def.DisplayName, len(landed), resourceIDs(landed))
				scenario.Press("esc")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestScenario_NavigableFieldDrillThrough_All
// ---------------------------------------------------------------------------

// TestScenario_NavigableFieldDrillThrough_All verifies that every registered
// navigable field on each graph-root fixture dispatches and lands on a non-empty
// resource. Subtests for resource types with no registered NavigableFields are
// skipped honestly — there is nothing to iterate.
func TestScenario_NavigableFieldDrillThrough_All(t *testing.T) {
	for _, tc := range drillThroughFixtures {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			navFields := resource.GetNavigableFields(tc.shortName)
			if len(navFields) == 0 {
				t.Skipf("no navigable fields registered for %q", tc.shortName)
				return
			}

			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			scenario := fullIntegrationNewDemoScenario(t)
			runDemoStartup(t, scenario)

			scenario.OpenList(tc.shortName)
			root := fullIntegrationMustFindResourceByID(t, scenario.clients, tc.shortName, tc.graphRoot)

			for _, nf := range navFields {
				// Open detail fresh before each navigable-field follow so the
				// resource's RawStruct is present and the stack is at detail level.
				scenario.OpenDetailResource(tc.shortName, root)
				scenario.ExpectNoAPIError()
				landed := scenario.FollowNavigableField(nf.FieldPath)
				if landed.ID == "" {
					t.Errorf("[%s] FollowNavigableField(%q → %s): empty landing",
						tc.label, nf.FieldPath, nf.TargetType)
					scenario.Press("esc")
					continue
				}
				t.Logf("[%s] FollowNavigableField(%q → %s): landed on %q",
					tc.label, nf.FieldPath, nf.TargetType, landed.ID)
				scenario.Press("esc")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers local to this file
// ---------------------------------------------------------------------------

// assertBareIDs verifies that every returned resource ID is already in bare
// form for the given target type. For target types with a registered NavID
// extractor (kms, role, ecs, logs, s3, iam-user), the extractor returns a
// non-empty value different from the input only when the input is an ARN.
// If the extractor's result differs from the input, the ID was an ARN — that
// is a format bug and this helper fails the test.
//
// For target types without a registered extractor (e.g. sns, whose IDs are
// ARNs by convention), NavIDFromValue returns the input unchanged — both
// branches below produce no failure, which is correct.
func assertBareIDs(t *testing.T, label, displayName, targetType string, landed []resource.Resource) {
	t.Helper()
	for _, r := range landed {
		if r.ID == "" {
			t.Errorf("[%s] %q: empty ID in landed result: %+v", label, displayName, r)
			continue
		}
		extracted := resource.NavIDFromValue(targetType, r.ID)
		// If extracted is non-empty AND different from r.ID, the extractor
		// converted an ARN to a bare ID — meaning r.ID was still an ARN.
		// That is the ID-format bug this helper is designed to catch.
		if extracted != "" && extracted != r.ID {
			t.Errorf("[%s] %q: landed ID %q is not in bare form for target %q (extractor produced %q)",
				label, displayName, r.ID, targetType, extracted)
		}
	}
}

// resourceIDs returns a slice of IDs from a resource slice for use in log messages.
func resourceIDs(resources []resource.Resource) []string {
	ids := make([]string, len(resources))
	for i, r := range resources {
		ids[i] = r.ID
	}
	return ids
}
