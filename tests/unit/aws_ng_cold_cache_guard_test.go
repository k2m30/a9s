// The NG→EC2 and NG→EBS checkers are cache-only: a missing "ec2" cache
// entry, or one holding struct-less disk-seeded rows, yields RelatedUnknown
// and never calls EC2:DescribeInstances. A struct-less row set cannot be
// tag-matched, so a resolved Count:0 would be a false negative in the
// RELATED panel.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

func TestNGColdCacheGuard_EC2_NoCacheEntry_NoLiveFetch(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := resource.ResourceCache{}
	clients := &awsclient.ServiceClients{EC2: &coldCacheGuardEC2Client{t: t}}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), clients, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ec2 cache entry, must not live-fetch)", result.Count())
	}
}

func TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := resource.ResourceCache{}
	clients := &awsclient.ServiceClients{EC2: &coldCacheGuardEC2Client{t: t}}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), clients, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ec2 cache entry, must not live-fetch)", result.Count())
	}
}

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
					// Fields-only rows are the shape the on-disk cache seeds after a restart.
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown) — struct-less disk-seeded ec2 cache rows cannot be tag-matched, must not report as an exact zero", result.Count())
	}
}

func TestNGColdCacheGuard_EBS_StructLessCacheRows_Unknown(t *testing.T) {
	source := ngColdCacheGuardSource(t)
	cache := ngColdCacheGuardStructLessEC2Cache()

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown) — struct-less disk-seeded ec2 cache rows cannot be tag-matched, must not report as an exact zero", result.Count())
	}
}
