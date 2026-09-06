package unit_test

// aws_lt_related_test.go — related-resource checker tests for lt (EC2 Launch
// Templates) (docs/resources/lt.md §2, docs/resources/lt-impl-plan.md §1
// "related_targets"). Checkers live in core/aws/lt_related.go.
//
// 5 pivots are Pattern F via *awsclient.LTRaw (ami, kms, sg, subnet — read a
// field on the $Default version, no API call). 3 are cache cross-ref
// (asg, ng, ec2 — scan the sibling ResourceCache, zero extra AWS calls).
// ct-events is the universal ctEventsCheckerFor("lt") pivot. role/eks are
// explicitly excluded per docs/resources/lt.md §2.
//
// checkerByTarget is shared package-scope test tooling, defined in
// aws_iam_policies_related_test.go. collectAllPages is shared package-scope
// test tooling, defined in helpers_external_test.go.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ltResourceByID fetches the real demo page via FetchLaunchTemplatesPage and
// returns the Resource with the given id, failing the test if absent. The
// demo set includes the details-denied witness (WarnLTDeniedID), so the
// fetch legitimately returns rows + a composite error (E5 partial success).
func ltResourceByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fakes.NewEC2(), "")
	if err != nil && !strings.Contains(err.Error(), fixtures.WarnLTDeniedID) {
		t.Fatalf("FetchLaunchTemplatesPage returned error: %v", err)
	}
	for _, r := range result.Resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// ltSiblingCache builds asg/ec2/ng ResourceCache entries from the REAL demo
// fixtures (the actual shipped fixture graph — asg.go/ec2.go/eks.go — rather
// than a synthetic stand-in), so the graph-root count tests validate the
// real cross-references confirmed in docs/resources/lt-impl-plan.md §2.
func ltSiblingCache(t *testing.T) resource.ResourceCache {
	t.Helper()
	ctx := context.Background()

	asgResources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchAutoScalingGroupsPage(ctx, fakes.NewASG(), token)
	})
	if err != nil {
		t.Fatalf("FetchAutoScalingGroupsPage: %v", err)
	}

	ec2Resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(ctx, fakes.NewEC2(), token)
	})
	if err != nil {
		t.Fatalf("FetchEC2InstancesPage: %v", err)
	}

	eksFake := fakes.NewEKS()
	pf := resource.GetPaginatedFetcher("ng")
	ngResult, err := pf(ctx, &awsclient.ServiceClients{EKS: eksFake, EC2: fakes.NewEC2()}, "")
	if err != nil && len(ngResult.Resources) == 0 {
		t.Fatalf("ng paginated fetcher: %v", err)
	}
	ngResources := ngResult.Resources

	return resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: asgResources},
		"ec2": resource.ResourceCacheEntry{Resources: ec2Resources},
		"ng":  resource.ResourceCacheEntry{Resources: ngResources},
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// TestRelated_LT_Registered pins the 8-target pivot SET from
// docs/resources/lt.md §2 (ami, asg, ec2, kms, ng, sg, subnet, ct-events) and
// that every one has a working, non-nil Checker. It deliberately does NOT
// pin exact DisplayName strings: docs/related-resources.md's generated table
// shows the wording is not uniform across resource types even for the SAME
// target (e.g. ec2's own registrations use "AMI"/"EKS Node Groups", other
// types use "AMIs"/"Node Groups") — DisplayName is per-registration free
// text, not a canonicalized value this suite enforces elsewhere.
func TestRelated_LT_Registered(t *testing.T) {
	defs := resource.GetRelated("lt")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for lt")
	}

	expectedTargets := []string{"ami", "asg", "ec2", "kms", "ng", "sg", "subnet", "ct-events"}
	if len(defs) != len(expectedTargets) {
		t.Errorf("lt: len(GetRelated) = %d, want exactly %d (spec §2's 8-target pivot SET must not gain an unlisted extra registration)", len(defs), len(expectedTargets))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		for _, want := range expectedTargets {
			if def.TargetType != want {
				continue
			}
			seen[want] = true
			if def.Checker == nil {
				t.Errorf("lt %q: Checker should not be nil", def.TargetType)
			}
			if def.DisplayName == "" {
				t.Errorf("lt %q: DisplayName should not be empty", def.TargetType)
			}
		}
	}
	for _, target := range expectedTargets {
		if !seen[target] {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_LT_ExcludedTargetsNotRegistered(t *testing.T) {
	defs := resource.GetRelated("lt")
	for _, excluded := range []string{"role", "eks"} {
		for _, def := range defs {
			if def.TargetType == excluded {
				t.Errorf("lt: target %q must not be registered (spec §2 explicitly excludes it)", excluded)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Pattern F graph-root counts (prod-web-lt): ami 1, kms 1, sg 2.
// ---------------------------------------------------------------------------

func TestRelated_LT_GraphRootPatternF(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)

	for _, tc := range []struct {
		target string
		want   int
		ids    []string
	}{
		{"ami", 1, []string{"ami-0a1b2c3d4e5f60001"}},
		{"kms", 1, []string{"a1b2c3d4-5678-90ab-cdef-111111111111"}},
		{"sg", 2, []string{"sg-0aaa111111111111a", "sg-0bbb222222222222b"}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checker := checkerByTarget(t, "lt", tc.target)
			result := checker(context.Background(), nil, res, resource.ResourceCache{})
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d", result.Count(), tc.want)
			}
			if !reflect.DeepEqual(result.ResourceIDs(), tc.ids) {
				t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs(), tc.ids)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cache cross-ref graph-root counts (prod-web-lt): asg 2, ec2 2 — against the
// REAL asg.go/ec2.go fixture cross-references.
// ---------------------------------------------------------------------------

func TestRelated_LT_GraphRootCacheCrossRef(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)
	cache := ltSiblingCache(t)

	for _, tc := range []struct {
		target string
		want   int
		ids    []string
	}{
		{"asg", 2, []string{"acme-staging-asg", "acme-worker-batch-asg"}},
		{"ec2", 2, []string{"i-0a1b2c3d4e5f60006", "i-0a1b2c3d4e5f60007"}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checker := checkerByTarget(t, "lt", tc.target)
			result := checker(context.Background(), nil, res, cache)
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d", result.Count(), tc.want)
			}
			if !reflect.DeepEqual(result.ResourceIDs(), tc.ids) {
				t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs(), tc.ids)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// eks-node-lt root: ng 1, subnet 1, sg 1 (NetworkInterfaces[] union — no
// top-level SecurityGroupIds/subnet fields on this fixture).
// ---------------------------------------------------------------------------

func TestRelated_LT_EKSNodeRootCounts(t *testing.T) {
	res := ltResourceByID(t, fixtures.EKSNodeLTID)

	t.Run("subnet", func(t *testing.T) {
		checker := checkerByTarget(t, "lt", "subnet")
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count() != 1 {
			t.Errorf("Count = %d, want 1", result.Count())
		}
		if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "subnet-0aaa111111111111a" {
			t.Errorf("ResourceIDs = %v, want [subnet-0aaa111111111111a]", result.ResourceIDs())
		}
	})

	t.Run("sg_via_network_interfaces_union", func(t *testing.T) {
		checker := checkerByTarget(t, "lt", "sg")
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count() != 1 {
			t.Errorf("Count = %d, want 1 (NetworkInterfaces[].Groups union)", result.Count())
		}
		if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sg-0bbb222222222222b" {
			t.Errorf("ResourceIDs = %v, want [sg-0bbb222222222222b]", result.ResourceIDs())
		}
	})

	t.Run("ng", func(t *testing.T) {
		checker := checkerByTarget(t, "lt", "ng")
		cache := ltSiblingCache(t)
		result := checker(context.Background(), nil, res, cache)
		if result.Count() != 1 {
			t.Errorf("Count = %d, want 1", result.Count())
		}
		if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "general-pool" {
			t.Errorf("ResourceIDs = %v, want [general-pool]", result.ResourceIDs())
		}
	})
}

// ---------------------------------------------------------------------------
// ssm_ami_no_pivot (related half) — a resolve:ssm: ImageId never resolves to
// an ami pivot (fetcher-side "no finding" half lives in aws_lt_test.go).
// ---------------------------------------------------------------------------

func TestRelated_LT_SSMReferenceNoAMIPivot(t *testing.T) {
	res := ltResourceByID(t, fixtures.SSMAmiLTID)
	checker := checkerByTarget(t, "lt", "ami")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (a resolve:ssm: reference is a display fact, never a pivot)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("Err = %v, want nil", result.Err())
	}
}

// ---------------------------------------------------------------------------
// truncated_cache_unknown — checkLTEC2 reads the sibling "ec2" cache
// directly (ltCachedEC2Instances), mirroring ng_related.go's
// ngCachedEC2Instances tri-state contract verbatim (lt_related.go's own
// header comment; see TestRelated_NG_EC2_TruncatedCacheNoMatch for the
// established precedent): cache ABSENT → RelatedUnknown ("?"); cache PRESENT
// and typed, even if truncated, is trusted enough to scan — a truncated
// cache with zero visible matches resolves to Count=0 with Truncated=true
// ("0+", not "?"), since the type-assertion success already confirms a real
// loaded ec2 list, not a disk-seeded stand-in.
// ---------------------------------------------------------------------------

func TestRelated_LT_EC2_TruncatedCacheResolvedNotUnknown(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			IsTruncated: true,
			Resources: []resource.Resource{
				{ID: "i-unrelated-0001", RawStruct: ec2types.Instance{
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("unrelated")}},
				}},
			},
		},
	}
	checker := checkerByTarget(t, "lt", "ec2")
	result := checker(context.Background(), nil, res, cache)
	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want RelatedResolved (a present, typed cache is trusted even when truncated)", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no visible match)", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (cache IsTruncated must carry through)")
	}
}

func TestRelated_LT_EC2_AbsentCacheUnknown(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)
	checker := checkerByTarget(t, "lt", "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (no ec2 cache entry at all)", result.State())
	}
}

// ---------------------------------------------------------------------------
// asg via MixedInstancesPolicy.LaunchTemplate.Overrides[] only — the
// per-instance-type override path, distinct from the top-level
// LaunchTemplate/MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
// fields already covered by the graph-root fixtures.
// ---------------------------------------------------------------------------

func TestRelated_LT_ASG_ViaOverridesOnly(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "override-only-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("override-only-asg"),
					MixedInstancesPolicy: &asgtypes.MixedInstancesPolicy{
						LaunchTemplate: &asgtypes.LaunchTemplate{
							// Top-level LaunchTemplateSpecification deliberately
							// absent — only a per-instance-type Overrides[] entry
							// references prod-web-lt.
							Overrides: []asgtypes.LaunchTemplateOverrides{
								{
									InstanceType: aws.String("m5.xlarge"),
									LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{
										LaunchTemplateId: aws.String(fixtures.ProdWebLTID),
									},
								},
							},
						},
					},
				},
			},
			{
				ID: "unrelated-asg",
				RawStruct: asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String("unrelated-asg"),
					LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String("lt-some-other-template"),
					},
				},
			},
		}},
	}

	checker := checkerByTarget(t, "lt", "asg")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (Overrides[].LaunchTemplateSpecification must resolve, not just the top-level fields)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "override-only-asg" {
		t.Errorf("ResourceIDs = %v, want [override-only-asg]", result.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// Degraded row (warn-lt-denied): Pattern-F pivots must resolve cleanly to 0
// against a zero-valued DefaultVersion — never Unknown (RawStruct type
// assertion still succeeds, it's the SAME *LTRaw type), never a panic.
// ---------------------------------------------------------------------------

func TestRelated_LT_DegradedRow_PatternFPivotsCleanZero(t *testing.T) {
	res := ltResourceByID(t, fixtures.WarnLTDeniedID)

	for _, target := range []string{"ami", "kms", "sg", "subnet"} {
		t.Run(target, func(t *testing.T) {
			checker := checkerByTarget(t, "lt", target)
			result := checker(context.Background(), nil, res, resource.ResourceCache{})
			if result.Err() != nil {
				t.Errorf("Err = %v, want nil (must not error/panic on a degraded row's zero DefaultVersion)", result.Err())
			}
			if result.Count() != 0 {
				t.Errorf("Count = %d, want 0 (degraded row has a zero DefaultVersion — no nested fields to pivot on)", result.Count())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ct-events — universal ctEventsCheckerFor("lt") pivot: deferred, server-side
// FetchFilter, drillable.
// ---------------------------------------------------------------------------

func TestRelated_LT_CtEvents_Drillable(t *testing.T) {
	res := ltResourceByID(t, fixtures.ProdWebLTID)
	checker := checkerByTarget(t, "lt", "ct-events")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedDeferred {
		t.Errorf("State = %v, want RelatedDeferred (ct-events is a universal server-side pivot, drillable by resource id)", result.State())
	}
	if len(result.FetchFilter()) == 0 {
		t.Error("FetchFilter is empty, want a CloudTrail LookupEvents filter keyed on the launch template id")
	}
}
