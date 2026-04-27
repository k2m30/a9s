package unit_test

// ctevent_projector_test.go — verifies that ctevent.Project wraps BuildSections
// faithfully and produces identical section/item content.
//
// This is the most visible regression risk in PR-01: ct-events detail is the only
// resource type that already had a custom rendering path. The move from
// internal/aws/ctdetail/ to internal/semantics/ctevent/ plus the wrapper to
// []domain.Section must not change a single byte of rendered output for any
// ct-event fixture.
//
// Note on fixture behavior:
//   - buildCTResource sets r.RawStruct to cloudtrailtypes.Event (not *ctevent.Event).
//   - ctevent.Project.parseResource first tries r.RawStruct.(*ctevent.Event) — that fails.
//   - It then tries r.Fields["raw"] — that is empty in demo fixtures.
//   - So ctevent.Project returns nil for raw demo fixtures.
//
// To test the projector, we construct domain.Resource values with RawStruct set
// to *ctevent.Event (the form ctevent.Project expects), by parsing the
// CloudTrailEvent JSON via ctevent.Parse.
//
// If ctevent.Project returns nil for a fixture, the test fatals — that is the
// correct failure signal during PR-01 development if the wrapper is incomplete.

import (
	"context"
	"fmt"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadCTEventFixtures loads all ct-event demo resources via the CloudTrail fake.
func loadCTEventFixtures(t *testing.T) []domain.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), clients.CloudTrail)
	if err != nil {
		t.Fatalf("FetchCloudTrailEvents: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchCloudTrailEvents returned no demo fixtures")
	}
	return resources
}

// buildProjectorResource converts a demo ct-event resource (RawStruct =
// cloudtrailtypes.Event) into a domain.Resource whose RawStruct is *ctevent.Event.
// This is the form that ctevent.Project expects for the fast path.
// Returns (resource, true) on success; (zero, false) when the fixture has no
// CloudTrailEvent JSON (bare stub).
func buildProjectorResource(t *testing.T, r domain.Resource) (domain.Resource, bool) {
	t.Helper()
	evt, ok := r.RawStruct.(cloudtrailtypes.Event)
	if !ok {
		t.Logf("fixture %q: RawStruct is %T, not cloudtrailtypes.Event — skipping", r.ID, r.RawStruct)
		return domain.Resource{}, false
	}
	if evt.CloudTrailEvent == nil || *evt.CloudTrailEvent == "" {
		t.Logf("fixture %q: CloudTrailEvent JSON is empty — skipping", r.ID)
		return domain.Resource{}, false
	}
	parsed, err := ctevent.Parse(*evt.CloudTrailEvent)
	if err != nil {
		t.Logf("fixture %q: ctevent.Parse failed: %v — skipping", r.ID, err)
		return domain.Resource{}, false
	}
	parsed.Status = r.Status // propagate severity tier from the Resource

	projectorResource := domain.Resource{
		ID:        r.ID,
		Name:      r.Name,
		Status:    r.Status,
		Fields:    r.Fields,
		RawStruct: parsed, // *ctevent.Event — ctevent.Project fast path
	}
	return projectorResource, true
}

// ---------------------------------------------------------------------------
// TestCTEventProjectorNonEmpty
// ---------------------------------------------------------------------------

// TestCTEventProjectorNonEmpty asserts that ctevent.Project returns a non-empty
// []domain.Section for every demo ct-event fixture when given a resource whose
// RawStruct is a *ctevent.Event.
//
// This is a liveness check: if the wrapper is a stub returning nil, every sub-test
// fails immediately, making the regression obvious before any rendered-output
// comparison is needed.
func TestCTEventProjectorNonEmpty(t *testing.T) {
	fixtures := loadCTEventFixtures(t)

	successCount := 0
	for _, r := range fixtures {
		r := r
		t.Run(r.ID, func(t *testing.T) {
			projRes, ok := buildProjectorResource(t, r)
			if !ok {
				t.Skip("fixture has no parseable CloudTrailEvent JSON")
			}

			sections := ctevent.Project(projRes)
			if len(sections) == 0 {
				t.Errorf("ctevent.Project(%q) returned zero sections; expected at least ACTION section", r.ID)
				return
			}
			successCount++
		})
	}

	if successCount == 0 && !t.Failed() {
		t.Error("all fixtures were skipped — no ct-event fixture produced projectable content")
	}
}

// ---------------------------------------------------------------------------
// TestCTEventProjectorMatchesBuildSections
// ---------------------------------------------------------------------------

// TestCTEventProjectorMatchesBuildSections asserts that ctevent.Project, when
// given a ct-event resource, returns a []domain.Section that matches the output
// of ctevent.BuildSections section-by-section and item-by-item.
//
// This is the most important regression guard in PR-01: ct-events detail is the
// only resource type with a custom rendering path. The semantics layer must
// preserve every field label, value, tier string, and navigability flag.
func TestCTEventProjectorMatchesBuildSections(t *testing.T) {
	fixtures := loadCTEventFixtures(t)

	for _, r := range fixtures {
		r := r
		t.Run(r.ID, func(t *testing.T) {
			// Build the *ctevent.Event from the raw fixture.
			evt, ok := r.RawStruct.(cloudtrailtypes.Event)
			if !ok || evt.CloudTrailEvent == nil || *evt.CloudTrailEvent == "" {
				t.Skip("fixture has no CloudTrailEvent JSON")
			}
			parsedEvent, err := ctevent.Parse(*evt.CloudTrailEvent)
			if err != nil {
				t.Skipf("ctevent.Parse failed: %v", err)
			}
			parsedEvent.Status = r.Status

			// Legacy path: BuildSections on the parsed event.
			legacySections := ctevent.BuildSections(parsedEvent)

			// New path: ctevent.Project on a domain.Resource with *ctevent.Event RawStruct.
			projRes, ok := buildProjectorResource(t, r)
			if !ok {
				t.Skip("could not build projector resource")
			}
			projSections := ctevent.Project(projRes)

			if len(projSections) == 0 && len(legacySections) > 0 {
				t.Fatalf("ctevent.Project returned zero sections but BuildSections returned %d; "+
					"the projector wrapper is incomplete",
					len(legacySections))
			}

			// Compare section count.
			if len(projSections) != len(legacySections) {
				t.Errorf("section count mismatch: ctevent.Project=%d, BuildSections=%d",
					len(projSections), len(legacySections))
				return
			}

			// Compare section by section.
			for i, legacySec := range legacySections {
				projSec := projSections[i]

				// Section title must match section name.
				if projSec.Title != legacySec.Name {
					t.Errorf("section[%d] title: got %q, want %q", i, projSec.Title, legacySec.Name)
				}

				// Item count must match row count.
				if len(projSec.Items) != len(legacySec.Rows) {
					t.Errorf("section[%d] (%q) item count: got %d, want %d",
						i, legacySec.Name, len(projSec.Items), len(legacySec.Rows))
					continue
				}

				// Item-by-item comparison.
				for j, legacyRow := range legacySec.Rows {
					projItem := projSec.Items[j]

					loc := fmt.Sprintf("section[%d](%q) item[%d](%q)", i, legacySec.Name, j, legacyRow.Key)

					// Label must match row Key.
					if projItem.Label != legacyRow.Key {
						t.Errorf("%s: Label got %q, want %q", loc, projItem.Label, legacyRow.Key)
					}

					// Value must match row Value.
					if projItem.Value != legacyRow.Value {
						t.Errorf("%s: Value got %q, want %q", loc, projItem.Value, legacyRow.Value)
					}

					// Tier string must be preserved exactly.
					if projItem.Tier != legacyRow.Severity {
						t.Errorf("%s: Tier got %q, want %q", loc, projItem.Tier, legacyRow.Severity)
					}

					// Navigable flag must match.
					if projItem.Navigable != legacyRow.IsNavigable {
						t.Errorf("%s: Navigable got %v, want %v", loc, projItem.Navigable, legacyRow.IsNavigable)
					}

					// TargetType must match when navigable.
					if legacyRow.IsNavigable && projItem.TargetType != legacyRow.TargetType {
						t.Errorf("%s: TargetType got %q, want %q", loc, projItem.TargetType, legacyRow.TargetType)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCTEventProjectorTierMapping
// ---------------------------------------------------------------------------

// TestCTEventProjectorTierMapping asserts that the tier-to-severity mapping in
// ctevent.Project is correct for the three tier strings used by ct-events:
//
//   - "ct-danger"     → domain.SevBroken
//   - "ct-attention"  → domain.SevWarn
//   - anything else   → domain.SevOK
//
// This exercises the convertRow path in ctevent/projector.go that maps
// ctevent Row.Severity (a tier string) to both domain.Item.Tier and domain.Item.Severity.
func TestCTEventProjectorTierMapping(t *testing.T) {
	cases := []struct {
		tier     string
		wantSev  domain.Severity
	}{
		{"ct-danger", domain.SevBroken},
		{"ct-attention", domain.SevWarn},
		{"ct-info", domain.SevOK},
		{"", domain.SevOK},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tier, func(t *testing.T) {
			// Build a minimal *ctevent.Event that will produce an ACTION section
			// with one row carrying the given tier.
			event := &ctevent.Event{
				EventID:     "test-event-id",
				EventSource: "s3.amazonaws.com",
				EventName:   "PutObject",
				Status:      tc.tier, // propagated to the ACTION section's Event row
			}

			projRes := domain.Resource{
				ID:        "test-event-id",
				Status:    tc.tier,
				Fields:    map[string]string{},
				RawStruct: event,
			}

			sections := ctevent.Project(projRes)
			if len(sections) == 0 {
				t.Fatalf("ctevent.Project returned zero sections for tier=%q; projector stub not yet implemented", tc.tier)
			}

			// Find the ACTION section Event row.
			for _, sec := range sections {
				if sec.Title != "ACTION" {
					continue
				}
				for _, item := range sec.Items {
					if item.Label != "Event" {
						continue
					}
					if item.Severity != tc.wantSev {
						t.Errorf("tier=%q: Action.Event.Severity got %v, want %v",
							tc.tier, item.Severity, tc.wantSev)
					}
					if item.Tier != tc.tier {
						t.Errorf("tier=%q: Action.Event.Tier got %q, want %q",
							tc.tier, item.Tier, tc.tier)
					}
					return
				}
			}
			t.Errorf("tier=%q: ACTION section or Event row not found in %d sections", tc.tier, len(sections))
		})
	}
}
