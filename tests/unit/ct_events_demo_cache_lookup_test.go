package unit_test

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestCtEventsCheckersResolveFromDemoCache(t *testing.T) {
	ctClient := fakes.NewCloudTrail()
	fixtures, fetchErr := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(context.Background(), ctClient, token)
	})
	if fetchErr != nil || len(fixtures) == 0 {
		t.Fatalf("demo ct-events fixtures missing (err=%v, len=%d)", fetchErr, len(fixtures))
	}

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — SetRelatedForTest not called?")
	}

	cacheBackedTypes := make(map[string]bool)
	for _, def := range defs {
		if def.NeedsTargetCache && def.TargetType != "ct-events" {
			cacheBackedTypes[def.TargetType] = true
		}
	}

	if len(cacheBackedTypes) == 0 {
		t.Skip("no NeedsTargetCache defs registered for ct-events — nothing to exercise")
	}

	// Target resource lists have not loaded yet.
	emptyCache := make(resource.ResourceCache)

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			allResults := ctEventsRealCheckerResults(fixture, emptyCache)

			for _, result := range allResults {
				if !cacheBackedTypes[result.TargetType()] {
					continue
				}

				// evt-* fixture events serve the generic ct-events->resource related
				// panel; their cache-backed typed checker stays unresolved with an empty
				// cache.
				if strings.HasPrefix(fixture.ID, "evt-") {
					continue
				}

				// A cache-backed checker reads the target list to answer. With
				// an empty cache and no client, nothing read that list, so
				// neither a count nor a zero is established and Unknown is the
				// only answer the row can carry.
				if result.State() != domain.RelatedUnknown && len(result.FetchFilter()) == 0 && result.Err() == nil {
					t.Errorf("event=%s targetType=%s: state = %v, Count=%d with nil clients and an empty cache,"+
						" want RelatedUnknown — no list was read, so a count is a guess dressed as a fact",
						fixture.ID, result.TargetType(), result.State(), result.Count())
				}
			}
		})
	}
}

func TestCtEventsCheckersResolveFromDemoCache_CaseKUserChecker(t *testing.T) {
	ctClient := fakes.NewCloudTrail()
	fixtures, fetchErr := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(context.Background(), ctClient, token)
	})
	if fetchErr != nil || len(fixtures) == 0 {
		t.Fatalf("demo ct-events fixtures missing (err=%v, len=%d)", fetchErr, len(fixtures))
	}

	var caseK resource.Resource
	var found bool
	for _, f := range fixtures {
		if f.ID == "e-e1f2a3b4" {
			caseK = f
			found = true
			break
		}
	}
	if !found {
		t.Skip("Case K fixture e-e1f2a3b4 not present in demo — skipping pinned regression")
	}

	if caseK.Fields["user"] != "alice.johnson" {
		t.Fatalf("Case K fixture user field=%q, want \"alice.johnson\"", caseK.Fields["user"])
	}

	// iam-user not yet loaded.
	emptyCache := make(resource.ResourceCache)

	allResults := ctEventsRealCheckerResults(caseK, emptyCache)

	var iamUserResult *resource.RelatedCheckResult
	for i, r := range allResults {
		if r.TargetType() == "iam-user" {
			cp := allResults[i]
			iamUserResult = &cp
			break
		}
	}
	if iamUserResult == nil {
		t.Fatal("no iam-user RelatedCheckResult returned for Case K — checker not registered?")
	}

	// With nothing to read and nothing to call, the checker has not
	// established that the user is absent, and a definitive zero would be a
	// guess dressed as a fact; Unknown is the required answer.
	if iamUserResult.State() != domain.RelatedUnknown {
		t.Errorf("event=e-e1f2a3b4 (AttachUserPolicy/alice.johnson): state = %v with nil clients and an empty cache,"+
			" want RelatedUnknown — nothing read the user list, so neither a count nor a zero is established."+
			" ResourceIDs=%v Err=%v FetchFilter=%v",
			iamUserResult.State(), iamUserResult.ResourceIDs(), iamUserResult.Err(), iamUserResult.FetchFilter())
	}
}

// AssumedRole events are identified by a non-empty role_name field.
// A role id in an event body is a claim about the past; with no list to
// confirm it against, Unknown is the only honest answer, and a confident 0
// would deny a role that exists.
func TestCtEventsCheckersResolveFromDemoCache_RoleCheckerAssumedRoleEvents(t *testing.T) {
	ctClient := fakes.NewCloudTrail()
	fixtures, fetchErr := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(context.Background(), ctClient, token)
	})
	if fetchErr != nil || len(fixtures) == 0 {
		t.Fatalf("demo ct-events fixtures missing (err=%v, len=%d)", fetchErr, len(fixtures))
	}

	emptyCache := make(resource.ResourceCache)

	type bugECase struct {
		fixtureID string
		count     int
	}
	var bugECases []bugECase

	var hasAssumedRoleEvent bool
	for _, fixture := range fixtures {
		if fixture.Fields["role_name"] == "" {
			continue // not an AssumedRole event
		}
		hasAssumedRoleEvent = true

		allResults := ctEventsRealCheckerResults(fixture, emptyCache)
		for _, r := range allResults {
			if r.TargetType() != "role" {
				continue
			}
			if r.State() != domain.RelatedUnknown && len(r.FetchFilter()) == 0 && r.Err() == nil {
				bugECases = append(bugECases, bugECase{
					fixtureID: fixture.ID,
					count:     r.Count(),
				})
			}
		}
	}

	if !hasAssumedRoleEvent {
		t.Log("INFO: no AssumedRole fixtures (role_name field set) in ct-events demo data — test is vacuous")
	}

	for _, bc := range bugECases {
		t.Run(bc.fixtureID, func(t *testing.T) {
			t.Errorf("event=%s: checkCtEventsRole answered Count=%d with nil clients and an"+
				" empty cache. With no list to confirm the named role against, the row must be"+
				" Unknown, not a count.",
				bc.fixtureID, bc.count)
		})
	}
}
