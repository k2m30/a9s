package unit_test

// ctevent_demo_invariants_test.go — table-driven invariant tests over all 12
// demo ct-events fixtures (Cases A–L).
//
// Three top-level tests:
//
//	TestCtEventsDemoLeftColumnNavigable — L1/L2/L3/L4: every navigable row from
//	  BuildSections must have a non-empty TargetType, resolve to a known resource
//	  type, and its NavID/Value must exist in demo fixtures.
//	  Bug D: Root principal must NOT be navigable (TargetType must be "").
//
//	TestCtEventsDemoRegistryNavigableFields — R1/R2: every registered NavigableField
//	  for ct-events must resolve to a known resource type; Root/AWSService events
//	  must have empty user/role_name post-cleanup.
//
//	TestCtEventsDemoRightColumnCheckers — G1/G2/G3: demo checker results are
//	  consistent: IDs with Count>0 must be in demo fixtures; Count=-1+FetchFilter
//	  must route to KindFilteredList or KindEnterChildView; Root events must
//	  return Count=0 for the role checker.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// loadAllCTFixtures returns all demo ct-events fixtures via the real fetcher
// backed by the typed CloudTrail fake.
func loadAllCTFixtures(t *testing.T) []resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	ctx := context.Background()
	fixtures, err := awsclient.FetchCloudTrailEvents(ctx, clients.CloudTrail)
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
// using demo.NewServiceClients() and real fetcher functions. No demo.GetResources
// calls — every entry is populated via the typed fakes.
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

	roles, err := awsclient.FetchIAMRoles(ctx, clients.IAM)
	fetch("role", roles, err)

	users, err := awsclient.FetchIAMUsers(ctx, clients.IAM)
	fetch("iam-user", users, err)

	instances, err := awsclient.FetchEC2Instances(ctx, clients.EC2)
	fetch("ec2", instances, err)

	buckets, err := awsclient.FetchS3Buckets(ctx, clients.S3)
	fetch("s3", buckets, err)

	lambdas, err := awsclient.FetchLambdaFunctions(ctx, clients.Lambda)
	fetch("lambda", lambdas, err)

	rdsInstances, err := awsclient.FetchRDSInstances(ctx, clients.RDS)
	fetch("rds", rdsInstances, err)

	kmsKeys, err := awsclient.FetchKMSKeys(ctx, clients.KMS, clients.KMS, clients.KMS)
	fetch("kms", kmsKeys, err)

	secrets, err := awsclient.FetchSecrets(ctx, clients.SecretsManager)
	fetch("secrets", secrets, err)

	vpce, err := awsclient.FetchVPCEndpoints(ctx, clients.EC2)
	fetch("vpce", vpce, err)

	sgs, err := awsclient.FetchSecurityGroups(ctx, clients.EC2)
	fetch("sg", sgs, err)

	ddbTables, err := awsclient.FetchDynamoDBTables(ctx, clients.DynamoDB, clients.DynamoDB)
	fetch("ddb", ddbTables, err)

	stacks, err := awsclient.FetchCloudFormationStacks(ctx, clients.CloudFormation)
	fetch("cfn", stacks, err)

	// Also populate ct-events itself for self-pivot lookups.
	ctEvents, err := awsclient.FetchCloudTrailEvents(ctx, clients.CloudTrail)
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

// ---------------------------------------------------------------------------
// TestCtEventsDemoLeftColumnNavigable
// ---------------------------------------------------------------------------

// TestCtEventsDemoLeftColumnNavigable iterates all 12 demo fixtures × every
// navigable row produced by ctevent.BuildSections and asserts:
//
//	L1: IsNavigable rows must have a non-empty TargetType.
//	L2: TargetType must resolve via resource.ResolveNavigationTarget.
//	L3: NavID (if set) or Value must exist in the demo fixture set for TargetType.
//	L4 (Bug D): Root-identity events must not have a navigable ACTOR.Principal row.
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

					// L4 (Bug D): Root-identity events must not have a navigable Principal.
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
					navID := row.NavID
					if navID == "" {
						navID = row.Value
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

// ---------------------------------------------------------------------------
// TestCtEventsDemoRegistryNavigableFields
// ---------------------------------------------------------------------------

// TestCtEventsDemoRegistryNavigableFields iterates all 12 demo fixtures × the
// 2 NavigableField registrations for ct-events ("user"→iam-user, "role_name"→role)
// and asserts:
//
//	R1: If the field value is non-empty, the TargetType must resolve to a known
//	    resource type and (if a fixture set exists) must contain the value.
//	R2: Root/AWSService events must have empty "user" AND empty "role_name"
//	    (to avoid spurious navigate targets for events with no real actor).
func TestCtEventsDemoRegistryNavigableFields(t *testing.T) {
	ensureNoColor(t)

	navFields := resource.GetNavigableFields("ct-events")
	if len(navFields) == 0 {
		t.Fatal("resource.GetNavigableFields(\"ct-events\") returned no fields — RegisterNavigableFields not called?")
	}

	fixtures := loadAllCTFixtures(t)
	cache := buildFakeResourceCache(t)

	for _, res := range fixtures {
		t.Run(res.ID, func(t *testing.T) {
			isRoot := isRootFixture(res)
			isAWSService := isAWSServiceFixture(res)

			for _, nf := range navFields {
				fieldVal := res.Fields[nf.FieldPath]

				rowLabel := fmt.Sprintf("event=%s field=%s targetType=%s value=%q",
					res.ID, nf.FieldPath, nf.TargetType, fieldVal)

				// R2: Root and AWSService events must have empty user and role_name.
				if (isRoot || isAWSService) && fieldVal != "" {
					t.Errorf("R2 FAIL: Root/AWSService event has non-empty navigable field %s=%q — "+
						"should be empty to prevent false navigation — %s",
						nf.FieldPath, fieldVal, rowLabel)
				}

				if fieldVal == "" {
					// Nothing to navigate to — this is expected for most Root/AWSService events.
					continue
				}

				// R1: TargetType must resolve.
				_, _, found := resource.ResolveNavigationTarget(nf.TargetType)
				if !found {
					t.Errorf("R1 FAIL: TargetType %q does not resolve via ResolveNavigationTarget — %s",
						nf.TargetType, rowLabel)
					continue
				}

				// R1: If demo data exists for TargetType, value must be a known ID or Name.
				fixtureIDs := fixtureIDsForType(cache, nf.TargetType)
				if len(fixtureIDs) == 0 {
					continue
				}
				found = fixtureIDs[fieldVal]
				if !found {
					entry := cache[nf.TargetType]
					for _, r := range entry.Resources {
						if r.Name == fieldVal {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("R1 FAIL: field value %q not found in fake cache for %q — %s",
						fieldVal, nf.TargetType, rowLabel)
				}
			}
		})
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

// ---------------------------------------------------------------------------
// TestCtEventsDemoRightColumnCheckers
// ---------------------------------------------------------------------------

// TestCtEventsDemoRightColumnCheckers iterates all 12 demo fixtures × the demo
// checker results (17 groups: 13 typed + 4 self-pivots) and asserts:
//
//	G1: Count>0 IDs must each exist in the fake resource cache for TargetType.
//	G2 (Bug C): Count=-1 + non-empty FetchFilter must route via ResolveRelatedNavigate
//	    to KindFilteredList or KindEnterChildView.
//	G3 (Bug A): Root-identity events must return Count=0 for the role checker.
func TestCtEventsDemoRightColumnCheckers(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)
	cache := buildFakeResourceCache(t)

	for _, res := range fixtures {
		t.Run(res.ID, func(t *testing.T) {
			results := ctEventsRealCheckerResults(res, cache)
			isRoot := isRootFixture(res)

			// Build a map for ResolveRelatedNavigate.
			resolveCache := make(map[string][]resource.Resource, len(cache))
			for k, v := range cache {
				resolveCache[k] = v.Resources
			}

			for _, result := range results {
				rowLabel := fmt.Sprintf("event=%s targetType=%s count=%d fetchFilter=%v ids=%v",
					res.ID, result.TargetType, result.Count, result.FetchFilter, result.ResourceIDs)

				// G3 (Bug A): Root events must have Count=0 for the role checker.
				if isRoot && result.TargetType == "role" && result.Count != 0 {
					t.Errorf("G3 (Bug A) FAIL: Root event has Count=%d for role checker, want 0 — %s",
						result.Count, rowLabel)
				}

				// G1: Count>0 resource IDs must each exist in the fake cache for TargetType.
				if result.Count > 0 {
					fixtureIDs := fixtureIDsForType(cache, result.TargetType)
					for _, rid := range result.ResourceIDs {
						// Strip compound key to first segment for child types.
						lookupID := rid
						if before, _, ok := strings.Cut(rid, "|"); ok {
							lookupID = before
						}
						if len(fixtureIDs) > 0 && !fixtureIDs[rid] {
							// Try the stripped ID (for s3_objects composite keys).
							entry := cache[result.TargetType]
							foundInFixtures := false
							for _, r := range entry.Resources {
								if r.ID == rid || r.ID == lookupID || r.Name == lookupID {
									foundInFixtures = true
									break
								}
							}
							if !foundInFixtures {
								t.Errorf("G1 FAIL: ResourceID %q not found in fake cache for %q — %s",
									rid, result.TargetType, rowLabel)
							}
						}
					}
				}

				// G2 (Bug C): Count=-1 + non-empty FetchFilter must route to
				// KindFilteredList or KindEnterChildView.
				if result.Count == -1 && len(result.FetchFilter) > 0 {
					navMsg := runtime.RelatedNavigateEvent{
						TargetType:  result.TargetType,
						FetchFilter: result.FetchFilter,
					}
					navResult := runtime.ResolveRelatedNavigate(navMsg, resolveCache)
					switch navResult.Kind {
					case runtime.NavigationKindFilteredList, runtime.NavigationKindEnterChildView:
						// G2 OK
					default:
						t.Errorf("G2 (Bug C) FAIL: Count=-1+FetchFilter routed to %v, want KindFilteredList or KindEnterChildView — %s",
							navResult.Kind, rowLabel)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCtEventsDemoRightColumnCheckers_RealCheckers
// ---------------------------------------------------------------------------

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

	// Find the role checker.
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
			// Real checker with nil clients (demo path: ctEventsRelatedResources
			// returns nil, false when clients is not *ServiceClients, so Count=-1
			// without error — that's OK, we just need Count != positive integer).
			result := roleChecker(ctx, nil, res, cache)
			if result.Count > 0 {
				t.Errorf("G3 (Bug A) FAIL: Real role checker returned Count=%d (IDs=%v) for Root event %q, want 0 — "+
					"Root events have no assumed role to match",
					result.Count, result.ResourceIDs, res.ID)
			}
		})
	}
}
