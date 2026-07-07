// aws_ng_cold_cache_guard_test.go pins the NG↔EC2 join's cache-only
// contract for BOTH registered checkers on source "ng" targeting "ec2" and
// "ebs" (resource.GetRelated("ng"), via ngCheckerByTarget from
// aws_ng_ebs_cache_join_test.go).
//
// Two scenarios, one per checker (4 tests total):
//
//   - No "ec2" cache entry at all: the checker must return
//     State: RelatedUnknown (resource.UnknownRelated) and must NEVER call
//     EC2:DescribeInstances — a cold/missing cache entry is not a live-fetch
//     trigger. Wired via a recording fake EC2 client inside a real
//     *aws.ServiceClients so any call fails the test immediately.
//   - An "ec2" cache entry present but holding disk-seeded rows (Fields
//     only, no RawStruct — the on-disk cache shape after a restart) must
//     also return State: RelatedUnknown, not a resolved Count:0 — a
//     struct-less row set cannot be tag-matched, so treating it as an exact
//     zero would be a false negative in the RELATED panel.
//
// RED today (HEAD): checkNGEC2 and checkNGEBS both call ngRelatedResources,
// which falls through to FetchRelatedTarget's live-fetch-on-cache-miss path
// and has no struct-shape guard on a cache hit.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// coldCacheGuardEC2Client fails the test if any EC2 API method is invoked.
// Embeds a nil awsclient.EC2API — any unoverridden method call panics with a
// nil-pointer dereference, which is an acceptable (loud) failure mode for a
// checker that must never reach the AWS client at all on a cache miss.
type coldCacheGuardEC2Client struct {
	awsclient.EC2API
	t *testing.T
}

func (m *coldCacheGuardEC2Client) DescribeInstances(
	_ context.Context,
	_ *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	m.t.Helper()
	m.t.Fatalf("DescribeInstances called — the NG<->EC2 cache join must never live-fetch EC2 on a cache miss")
	return nil, nil
}

func ngColdCacheGuardSource(t *testing.T) resource.Resource {
	t.Helper()
	const ngName = "cold-cache-pool"
	const clusterName = "cold-cache-cluster"
	return resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
			"cluster_name":   clusterName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			ClusterName:   aws.String(clusterName),
		},
	}
}

// --- Scenario (a): no "ec2" cache entry at all, recording fake EC2 client ---

func TestNGColdCacheGuard_EC2_NoCacheEntry_NoLiveFetch(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := resource.ResourceCache{}
	clients := &awsclient.ServiceClients{EC2: &coldCacheGuardEC2Client{t: t}}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), clients, source, cache)

	if result.State != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ec2 cache entry, must not live-fetch)", result.Count)
	}
}

func TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := resource.ResourceCache{}
	clients := &awsclient.ServiceClients{EC2: &coldCacheGuardEC2Client{t: t}}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), clients, source, cache)

	if result.State != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ec2 cache entry, must not live-fetch)", result.Count)
	}
}

// --- Scenario (b): "ec2" cache entry present but rows are struct-less
// (Fields only, no RawStruct — the disk-seed shape) ---

func ngColdCacheGuardStructLessEC2Cache() resource.ResourceCache {
	return resource.ResourceCache{
		"ec2": {
			Resources: []resource.Resource{
				{
					ID:   "i-0diskseeded000001",
					Name: "i-0diskseeded000001",
					Fields: map[string]string{
						"instance_id": "i-0diskseeded000001",
						"state":       "running",
					},
					// No RawStruct — the on-disk cache seed shape.
				},
			},
			IsTruncated: false,
		},
	}
}

func TestNGColdCacheGuard_EC2_StructLessCacheRows_Unknown(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := ngColdCacheGuardStructLessEC2Cache()

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.State != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown) — struct-less disk-seeded ec2 cache rows cannot be tag-matched, must not report as an exact zero", result.Count)
	}
}

func TestNGColdCacheGuard_EBS_StructLessCacheRows_Unknown(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := ngColdCacheGuardStructLessEC2Cache()

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.State != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown) — struct-less disk-seeded ec2 cache rows cannot be tag-matched, must not report as an exact zero", result.Count)
	}
}
