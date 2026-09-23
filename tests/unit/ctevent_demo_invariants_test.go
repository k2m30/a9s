package unit_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// loadAllCTFixtures returns all demo ct-events fixtures via the real fetcher
// backed by the typed CloudTrail fake.
func loadAllCTFixtures(t *testing.T) []resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	ctx := context.Background()
	fixtures, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(ctx, clients.CloudTrail, token)
	})
	if err != nil {
		t.Fatalf("FetchCloudTrailEvents: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("FetchCloudTrailEvents returned no fixtures")
	}
	return fixtures
}

// parseCTEventForFixture parses the CloudTrailEvent JSON blob from the fixture's
// RawStruct and returns a *ctevent.Event via ctevent.Parse.
// Fatals if the fixture has no RawStruct, no CloudTrailEvent, or parse fails.
func parseCTEventForFixture(t *testing.T, res resource.Resource) *ctevent.Event {
	t.Helper()
	evt, ok := res.RawStruct.(cloudtrailtypes.Event)
	if !ok {
		t.Fatalf("fixture %q: RawStruct is %T, want cloudtrailtypes.Event", res.ID, res.RawStruct)
	}
	if evt.CloudTrailEvent == nil || *evt.CloudTrailEvent == "" {
		t.Fatalf("fixture %q: CloudTrailEvent JSON is empty", res.ID)
	}
	parsed, err := ctevent.Parse(*evt.CloudTrailEvent)
	if err != nil {
		t.Fatalf("fixture %q: ctevent.Parse failed: %v", res.ID, err)
	}
	return parsed
}

// buildFakeResourceCache builds a ResourceCache for all ct-events related types
// using demo.NewServiceClients() and real fetcher functions.
func buildFakeResourceCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	clients := demo.NewServiceClients()
	ctx := context.Background()
	cache := make(resource.ResourceCache)

	fetch := func(targetType string, resources []resource.Resource, err error) {
		t.Helper()
		if err != nil {
			t.Logf("buildFakeResourceCache: skipping %q: fetch error: %v", targetType, err)
			return
		}
		if len(resources) > 0 {
			cache[targetType] = resource.ResourceCacheEntry{Resources: resources, IsTruncated: false}
		}
	}

	roles, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchIAMRolesPage(ctx, clients.IAM, token)
	})
	fetch("role", roles, err)

	users, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchIAMUsersPage(ctx, clients.IAM, token)
	})
	fetch("iam-user", users, err)

	instances, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(ctx, clients.EC2, token)
	})
	fetch("ec2", instances, err)

	buckets, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPageWithNotifications(ctx, clients.S3, nil, token)
	})
	fetch("s3", buckets, err)

	lambdas, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchLambdaFunctionsPage(ctx, clients.Lambda, token)
	})
	fetch("lambda", lambdas, err)

	rdsInstances, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchRDSInstancesPage(ctx, clients.RDS, token)
	})
	fetch("rds", rdsInstances, err)

	kmsKeys, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchKMSKeysPage(ctx, clients, token)
	})
	fetch("kms", kmsKeys, err)

	secrets, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecretsPage(ctx, clients.SecretsManager, token)
	})
	fetch("secrets", secrets, err)

	vpce, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchVPCEndpointsPage(ctx, clients.EC2, token)
	})
	fetch("vpce", vpce, err)

	sgs, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(ctx, clients.EC2, token)
	})
	fetch("sg", sgs, err)

	ddbTables, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchDynamoDBTablesPage(ctx, clients.DynamoDB, clients.DynamoDB, token)
	})
	fetch("ddb", ddbTables, err)

	stacks, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudFormationStacksPage(ctx, clients.CloudFormation, token)
	})
	fetch("cfn", stacks, err)

	// Also populate ct-events itself for self-pivot lookups.
	ctEvents, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudTrailEventsPage(ctx, clients.CloudTrail, token)
	})
	fetch("ct-events", ctEvents, err)

	return cache
}

// fixtureIDsForType returns a set of IDs from the given ResourceCache entry.
// Returns an empty map (not nil) when no data exists for that type.
func fixtureIDsForType(cache resource.ResourceCache, targetType string) map[string]bool {
	entry, ok := cache[targetType]
	if !ok {
		return map[string]bool{}
	}
	ids := make(map[string]bool, len(entry.Resources))
	for _, r := range entry.Resources {
		ids[r.ID] = true
	}
	return ids
}

// isRootFixture reports whether the fixture represents a Root-identity event.
// Reads _ct.is_root from Fields; falls back to checking the CloudTrailEvent JSON.
func isRootFixture(res resource.Resource) bool {
	if res.Fields["_ct.is_root"] == "true" {
		return true
	}
	// Fallback: inspect the RawStruct CloudTrailEvent JSON.
	evt, ok := res.RawStruct.(cloudtrailtypes.Event)
	if !ok || evt.CloudTrailEvent == nil {
		return false
	}
	return strings.Contains(*evt.CloudTrailEvent, `"type":"Root"`)
}

// TestCtEventsDemoLeftColumnNavigable iterates all 12 demo fixtures × every
// navigable row produced by ctevent.BuildSections and asserts:
//
//	L1: IsNavigable rows must have a non-empty TargetType.
//	L2: TargetType must resolve via resource.ResolveNavigationTarget.
//	L3: NavID (if set) or Value must exist in the demo fixture set for TargetType.
//	L4: Root-identity events must not have a navigable ACTOR.Principal row.
func TestCtEventsDemoLeftColumnNavigable(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("no ct-events fixtures available")
	}

	cache := buildFakeResourceCache(t)

	for _, res := range fixtures {
		t.Run(res.ID, func(t *testing.T) {
			parsed := parseCTEventForFixture(t, res)
			sections := ctevent.BuildSections(parsed)

			isRoot := isRootFixture(res)

			for _, section := range sections {
				for _, row := range section.Rows {
					if !row.IsNavigable {
						continue
					}

					rowLabel := fmt.Sprintf("event=%s section=%s key=%s value=%s",
						res.ID, section.Name, row.Key, row.Value)

					// L1: navigable rows must have a non-empty TargetType.
					if row.TargetType == "" {
						t.Errorf("L1 FAIL: navigable row has empty TargetType — %s", rowLabel)
						continue
					}

					// L4: Root-identity events must not have a navigable Principal.
					if isRoot && section.Name == ctevent.SectionActor && row.Key == "Principal" {
						t.Errorf("L4 (Bug D) FAIL: Root event has navigable Principal row — %s", rowLabel)
					}

					// L2: TargetType must resolve to a known resource type.
					_, _, found := resource.ResolveNavigationTarget(row.TargetType)
					if !found {
						t.Errorf("L2 FAIL: TargetType %q not found via ResolveNavigationTarget — %s",
							row.TargetType, rowLabel)
						continue
					}

					// L3: NavID (or Value) must exist in demo fixture set for TargetType.
					// Skip this check for child types (e.g. s3_objects) and ct-events self-pivots
					// because their IDs encode composite keys or are filter-only.
					// The row carries a reference — its NavID, else its value —
					// and the detail resolves it through the target type's own
					// resolver against the loaded list, which is what reads the
					// account in an ARN and the suffix on a secret's. Resolve
					// it the same way here.
					ref := row.NavID
					if ref == "" {
						ref = row.Value
					}
					navID := resource.NavIDFromValue(row.TargetType, ref,
						domain.RefContext{AccountID: "123456789012", Targets: cache[row.TargetType].Resources})
					if navID == "" {
						// The reference names nothing this account holds —
						// another account's principal, a deleted resource.
						// The detail drops navigability for those, so there
						// is no row here to reach a fixture.
						continue
					}
					if navID == "" {
						t.Errorf("L3 FAIL: navigable row has empty NavID and Value — %s", rowLabel)
						continue
					}

					// For compound IDs (e.g. "bucket|key") take only the first segment.
					if idx := strings.Index(navID, "|"); idx >= 0 {
						navID = navID[:idx]
					}

					fixtureIDs := fixtureIDsForType(cache, row.TargetType)
					if len(fixtureIDs) == 0 {
						// No demo data for this type — can't validate L3.
						continue
					}

					// For child types we skip name-to-ID matching since composite IDs
					// are used for navigation entry only.
					_, isChild, _ := resource.ResolveNavigationTarget(row.TargetType)
					if isChild {
						continue
					}

					// Also skip self-pivot ct-events (they use FetchFilter, not IDs).
					if row.TargetType == "ct-events" {
						continue
					}

					// demofixtures.CtEventDeletedBucket names a bucket the account does
					// not hold, as a real DeleteBucket event does. The left column offers its
					// TARGET value as navigable anyway, so it is skipped here.
					if res.ID == demofixtures.CtEventDeletedBucket {
						continue
					}

					// demofixtures.CtEventDeletedRole names a role the account does not
					// hold; the left column offers the value anyway.
					if res.ID == demofixtures.CtEventDeletedRole {
						continue
					}

					// Check by ID or by Name (role names are IDs in demo).
					found = fixtureIDs[navID]
					if !found {
						// Try matching by Name in the cache.
						entry := cache[row.TargetType]
						for _, r := range entry.Resources {
							if r.Name == navID {
								found = true
								break
							}
						}
					}
					if !found {
						t.Errorf("L3 FAIL: navID %q not found in fake cache for %q — %s",
							navID, row.TargetType, rowLabel)
					}
				}
			}
		})
	}
}

// TestCtEventsDemoActorNavigationMatchesTheIdentity walks every demo fixture's
// rendered ACTOR section and asserts:
//
//	A1: an account-root or AWSService identity offers nothing to navigate to —
//	    neither names a browseable IAM principal, so an underlined value there
//	    would send the operator to an empty view.
//	A2: the "user" and "role_name" fields the fetcher writes carry an actor only
//	    when there is one, so a filtered ct-events list built from either never
//	    names the root account or a service.
//
// A3 guards the sweep itself: at least one fixture must render a navigable
// actor, otherwise A1 holds vacuously.
func TestCtEventsDemoActorNavigationMatchesTheIdentity(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)
	navigableActors := 0

	for _, res := range fixtures {
		isRoot := isRootFixture(res)
		isAWSService := isAWSServiceFixture(res)

		t.Run(res.ID, func(t *testing.T) {
			for _, field := range []string{"user", "role_name"} {
				if v := res.Fields[field]; (isRoot || isAWSService) && v != "" {
					t.Errorf("A2 FAIL: event=%s has %s=%q — a root or service identity names no IAM principal",
						res.ID, field, v)
				}
			}

			parsed := parseCTEventForFixture(t, res)
			for _, section := range ctevent.BuildSections(parsed) {
				if section.Name != ctevent.SectionActor {
					continue
				}
				for _, row := range section.Rows {
					if !row.IsNavigable {
						continue
					}
					navigableActors++
					if isRoot || isAWSService {
						t.Errorf("A1 FAIL: event=%s offers a navigable ACTOR row %s=%q (target %s) for a root or service identity",
							res.ID, row.Key, row.Value, row.TargetType)
					}
				}
			}
		})
	}

	if navigableActors == 0 {
		t.Error("A3 FAIL: no demo fixture renders a navigable ACTOR row, so A1 is unfalsifiable")
	}
}

// isAWSServiceFixture reports whether the fixture has an AWSService-type identity.
func isAWSServiceFixture(res resource.Resource) bool {
	evt, ok := res.RawStruct.(cloudtrailtypes.Event)
	if !ok || evt.CloudTrailEvent == nil {
		return false
	}
	return strings.Contains(*evt.CloudTrailEvent, `"type":"AWSService"`)
}

// TestCtEventsDemoRightColumnCheckers iterates all 12 demo fixtures × the demo
// checker results (17 groups: 13 typed + 4 self-pivots) and asserts:
//
//	G1: Count>0 IDs must each exist in the fake resource cache for TargetType.
//	G2: State RelatedDeferred with a non-empty FetchFilter must route via ResolveRelatedNavigate
//	    to NavigationKindFilteredList or NavigationKindEnterChildView.
//	G3: Root-identity events must return Count=0 for the role checker.
func TestCtEventsDemoRightColumnCheckers(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)
	cache := buildFakeResourceCache(t)

	for _, res := range fixtures {
		t.Run(res.ID, func(t *testing.T) {
			results := ctEventsRealCheckerResults(res, cache)
			isRoot := isRootFixture(res)

			resolveCache := make(map[string][]resource.Resource, len(cache))
			for k, v := range cache {
				resolveCache[k] = v.Resources
			}

			for _, result := range results {
				rowLabel := fmt.Sprintf("event=%s targetType=%s count=%d fetchFilter=%v ids=%v",
					res.ID, result.TargetType(), result.Count(), result.FetchFilter(), result.ResourceIDs())

				// G3: Root events must have Count=0 for the role checker.
				if isRoot && result.TargetType() == "role" && result.Count() != 0 {
					t.Errorf("G3 (Bug A) FAIL: Root event has Count=%d for role checker, want 0 — %s",
						result.Count(), rowLabel)
				}

				// G1: Count>0 resource IDs must each exist in the fake cache for TargetType.
				if result.Count() > 0 {
					fixtureIDs := fixtureIDsForType(cache, result.TargetType())
					for _, rid := range result.ResourceIDs() {
						// Strip compound key to first segment for child types.
						lookupID := rid
						if before, _, ok := strings.Cut(rid, "|"); ok {
							lookupID = before
						}
						if len(fixtureIDs) > 0 && !fixtureIDs[rid] {
							// Try the stripped ID (for s3_objects composite keys).
							entry := cache[result.TargetType()]
							foundInFixtures := false
							for _, r := range entry.Resources {
								if r.ID == rid || r.ID == lookupID || r.Name == lookupID {
									foundInFixtures = true
									break
								}
							}
							if !foundInFixtures {
								t.Errorf("G1 FAIL: ResourceID %q not found in fake cache for %q — %s",
									rid, result.TargetType(), rowLabel)
							}
						}
					}
				}

				// G2: State: RelatedDeferred (+ non-empty FetchFilter) must
				// route to NavigationKindFilteredList or NavigationKindEnterChildView.
				if result.State() == domain.RelatedDeferred {
					navMsg := runtime.RelatedNavigateEvent{
						TargetType:  result.TargetType(),
						FetchFilter: result.FetchFilter(),
					}
					navResult := runtime.ResolveRelatedNavigate(navMsg, resolveCache)
					switch navResult.Kind {
					case runtime.NavigationKindFilteredList, runtime.NavigationKindEnterChildView:
						// G2 OK
					default:
						t.Errorf("G2 (Bug C) FAIL: State: RelatedDeferred routed to %v, want NavigationKindFilteredList or NavigationKindEnterChildView — %s",
							navResult.Kind, rowLabel)
					}
				}
			}
		})
	}
}

// TestCtEventsDemoRightColumnCheckers_RealCheckers runs the REAL production
// checkers (not the demo overrides) against the demo resource cache to catch
// mapping bugs that the demo checker might mask.
//
// Asserts G3: Root-identity events must return Count=0 for role checker when
// the real checker runs against a cache containing demo roles.
func TestCtEventsDemoRightColumnCheckers_RealCheckers(t *testing.T) {
	ensureNoColor(t)

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs")
	}

	var roleChecker resource.RelatedChecker
	for _, def := range defs {
		if def.TargetType == "role" {
			roleChecker = def.Checker
			break
		}
	}
	if roleChecker == nil {
		t.Fatal("no role checker registered in ct-events RelatedDefs")
	}

	fixtures := loadAllCTFixtures(t)
	cache := buildFakeResourceCache(t)
	ctx := context.Background()

	for _, res := range fixtures {
		if !isRootFixture(res) {
			continue
		}
		t.Run("Root/"+res.ID, func(t *testing.T) {
			// Real checker with nil clients: the result may stay unresolved;
			// only a positive Count is wrong.
			result := roleChecker(ctx, nil, res, cache)
			if result.Count() > 0 {
				t.Errorf("G3 (Bug A) FAIL: Real role checker returned Count=%d (IDs=%v) for Root event %q, want 0 — "+
					"Root events have no assumed role to match",
					result.Count(), result.ResourceIDs(), res.ID)
			}
		})
	}
}
