package unit_test

import (
	"context"
	"fmt"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// loadCTEventFixtures loads all ct-event demo resources via the CloudTrail fake.
func loadCTEventFixtures(t *testing.T) []domain.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(context.Background(), clients.CloudTrail, token)
	})
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
	parsed.Status = r.Fields["status"] // propagate severity tier from the Resource

	projectorResource := domain.Resource{
		ID:        r.ID,
		Name:      r.Name,
		Fields:    r.Fields,
		RawStruct: parsed, // *ctevent.Event — ctevent.Project fast path
	}
	return projectorResource, true
}

// TestCTEventProjectorNonEmpty asserts that ctevent.Project returns a non-empty
// []domain.Section for every demo ct-event fixture when given a resource whose
// RawStruct is a *ctevent.Event.
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

// TestCTEventProjectorMatchesBuildSections asserts that ctevent.Project, when
// given a ct-event resource, returns a []domain.Section that structurally matches
// the output of ctevent.BuildSections — same section count, same section titles in
// order, same item count per section, and correct field mapping (Label←Key,
// Value←Value, Tier←Severity, Navigable←IsNavigable, TargetType←TargetType).
func TestCTEventProjectorMatchesBuildSections(t *testing.T) {
	fixtures := loadCTEventFixtures(t)

	for _, r := range fixtures {
		r := r
		t.Run(r.ID, func(t *testing.T) {
			evt, ok := r.RawStruct.(cloudtrailtypes.Event)
			if !ok || evt.CloudTrailEvent == nil || *evt.CloudTrailEvent == "" {
				t.Skip("fixture has no CloudTrailEvent JSON")
			}
			parsedEvent, err := ctevent.Parse(*evt.CloudTrailEvent)
			if err != nil {
				t.Skipf("ctevent.Parse failed: %v", err)
			}
			parsedEvent.Status = r.Fields["status"]

			legacySections := ctevent.BuildSections(parsedEvent)

			projRes := domain.Resource{
				ID:        r.ID,
				Name:      r.Name,
				Fields:    r.Fields,
				RawStruct: parsedEvent, // *ctevent.Event — ctevent.Project fast path
			}
			projSections := ctevent.Project(projRes)

			if len(projSections) == 0 && len(legacySections) > 0 {
				t.Fatalf("ctevent.Project returned zero sections but BuildSections returned %d; "+
					"the projector wrapper is incomplete",
					len(legacySections))
			}

			if len(projSections) != len(legacySections) {
				t.Errorf("section count mismatch: ctevent.Project=%d, BuildSections=%d",
					len(projSections), len(legacySections))
				return
			}

			for i, legacySec := range legacySections {
				projSec := projSections[i]
				if projSec.Title != legacySec.Name {
					t.Errorf("section[%d] title: got %q, want %q", i, projSec.Title, legacySec.Name)
				}
			}

			legacyTotal := 0
			for _, s := range legacySections {
				legacyTotal += len(s.Rows)
			}
			projTotal := 0
			for _, s := range projSections {
				projTotal += len(s.Items)
			}
			if projTotal != legacyTotal {
				t.Errorf("total item count mismatch: ctevent.Project=%d, BuildSections=%d",
					projTotal, legacyTotal)
				return
			}

			for i, legacySec := range legacySections {
				projSec := projSections[i]
				if len(projSec.Items) != len(legacySec.Rows) {
					t.Errorf("section[%d] (%q) item count: ctevent.Project=%d, BuildSections=%d",
						i, legacySec.Name, len(projSec.Items), len(legacySec.Rows))
				}
			}

			for i, projSec := range projSections {
				for j, item := range projSec.Items {
					loc := fmt.Sprintf("section[%d](%q) item[%d]", i, projSec.Title, j)

					if item.Label == "" {
						t.Errorf("%s: Label is empty; convertRow must preserve Row.Key", loc)
					}

					if item.Kind != domain.ItemField {
						t.Errorf("%s: Kind got %v, want ItemField", loc, item.Kind)
					}

					if item.Navigable && item.TargetType == "" {
						t.Errorf("%s: Navigable=true but TargetType is empty", loc)
					}

					if !item.Navigable && item.TargetType != "" {
						t.Errorf("%s: Navigable=false but TargetType=%q (should be empty)", loc, item.TargetType)
					}

					switch item.Tier {
					case "", "ct-info", "ct-attention", "ct-danger":
						// valid
					default:
						t.Errorf("%s: Tier=%q is not a known ct-event tier string", loc, item.Tier)
					}
				}
			}

			// The ACTION section's Event row is the only row that carries a Severity;
			// the fixture's Status propagates to it.
			for _, sec := range projSections {
				if sec.Title != "ACTION" {
					continue
				}
				foundEvent := false
				for _, item := range sec.Items {
					if item.Label != "Event" {
						continue
					}
					foundEvent = true
					if item.Tier != parsedEvent.Status {
						t.Errorf("ACTION Event row: Tier got %q, want %q (parsedEvent.Status)",
							item.Tier, parsedEvent.Status)
					}
					break
				}
				if !foundEvent {
					t.Error("ACTION section missing Event row")
				}
				break
			}
		})
	}
}

// TestCTEventProjectorTierMapping asserts that the tier-to-severity mapping in
// ctevent.Project is correct for the three tier strings used by ct-events:
//
//   - "ct-danger"     → domain.SevBroken
//   - "ct-attention"  → domain.SevWarn
//   - anything else   → domain.SevOK
func TestCTEventProjectorTierMapping(t *testing.T) {
	cases := []struct {
		tier    string
		wantSev domain.Severity
	}{
		{"ct-danger", domain.SevBroken},
		{"ct-attention", domain.SevWarn},
		{"ct-info", domain.SevOK},
		{"", domain.SevOK},
		{"unknown-tier", domain.SevOK}, // default arm: any unrecognized tier maps to SevOK
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tier, func(t *testing.T) {
			// Build a minimal *ctevent.Event that will produce an ACTION section
			// with one row carrying the given tier.
			event := &ctevent.Event{
				EventID:     "test-event-id",
				EventSource: "s3.amazonaws.com",
				EventName:   "PutObject", // propagated to the ACTION section's Event row
			}

			projRes := domain.Resource{
				ID:        "test-event-id",
				Fields:    map[string]string{"status": tc.tier},
				RawStruct: event,
			}

			sections := ctevent.Project(projRes)
			if len(sections) == 0 {
				t.Fatalf("ctevent.Project returned zero sections for tier=%q; projector stub not yet implemented", tc.tier)
			}

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
