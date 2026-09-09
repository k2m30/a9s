package unit_test

// rel2_redis_dbc_subnet_test.go — rows 1 and 2: the two-hop subnet checkers
// answer "there is none" and "we could not look" with the same Unknown, and a
// replication group with no members leaves its four member-driven pivots
// without a count at all.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// rel2RedisGroup builds a replication group with the given member cluster
// names. An empty list is a group AWS reports as having no members.
func rel2RedisGroup(members ...string) resource.Resource {
	return resource.Resource{
		ID:   "acme-redis-sessions",
		Name: "acme-redis-sessions",
		Type: "redis",
		RawStruct: elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("acme-redis-sessions"),
			MemberClusters:     members,
			Status:             aws.String("available"),
		},
	}
}

// rel2RedisSubnetCache holds a populated subnet list, so an empty match is the
// checker's answer rather than a missing target list.
func rel2RedisSubnetCache() resource.ResourceCache {
	return resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "subnet-prod-a", Fields: map[string]string{}},
			{ID: "subnet-prod-b", Fields: map[string]string{}},
		}},
	}
}

// --- row 1: three answers, three results ------------------------------------

// TestRel2RedisSubnetNoMemberIsAProvenZero pins the first of the three answers
// the checker collapses today: a replication group with no member cluster has
// no subnet group to reach, which is a fact about the group and not a gap in
// what we could read.
func TestRel2RedisSubnetNoMemberIsAProvenZero(t *testing.T) {
	clients := &awsclient.ServiceClients{ElastiCache: &mockElastiCacheFullAPI{}}
	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2RedisGroup(), rel2RedisSubnetCache())

	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want Resolved: a group with no members has no subnets to find", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// TestRel2RedisSubnetMemberWithoutGroupIsAProvenZero pins the second answer: a
// member cluster that names no subnet group is a cluster outside a VPC, which
// is again a fact rather than a gap.
func TestRel2RedisSubnetMemberWithoutGroupIsAProvenZero(t *testing.T) {
	clients := &awsclient.ServiceClients{ElastiCache: &mockElastiCacheFullAPI{
		cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
			CacheClusters: []elasticachetypes.CacheCluster{{
				CacheClusterId:       aws.String("acme-redis-sessions-001"),
				CacheSubnetGroupName: nil,
			}},
		},
	}}
	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2RedisGroup("acme-redis-sessions-001"), rel2RedisSubnetCache())

	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want Resolved: a member naming no subnet group has no subnets", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// TestRel2RedisSubnetCallFailureIsAnError pins the third answer: a describe
// that failed is not a zero and not an unknown shape — it is the error, so the
// panel can say what went wrong instead of showing a bare "?".
func TestRel2RedisSubnetCallFailureIsAnError(t *testing.T) {
	clients := &awsclient.ServiceClients{ElastiCache: &mockElastiCacheFullAPI{
		cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
			CacheClusters: []elasticachetypes.CacheCluster{{
				CacheClusterId:       aws.String("acme-redis-sessions-001"),
				CacheSubnetGroupName: aws.String("acme-redis-subnet-group"),
			}},
		},
		cacheSubnetGroupsErr: errors.New("AccessDenied: elasticache:DescribeCacheSubnetGroups"),
	}}
	checker := redisCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2RedisGroup("acme-redis-sessions-001"), rel2RedisSubnetCache())

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want Error", result.State())
	}
	if result.Err() == nil {
		t.Errorf("Err = nil; the panel has nothing to show the operator")
	}
}

// rel2DbcCluster builds an Aurora cluster naming the given subnet group. An
// empty name is a cluster that names none.
func rel2DbcCluster(subnetGroup string) resource.Resource {
	cluster := rdstypes.DBCluster{DBClusterIdentifier: aws.String("acme-aurora-prod")}
	if subnetGroup != "" {
		cluster.DBSubnetGroup = aws.String(subnetGroup)
	}
	return resource.Resource{ID: "acme-aurora-prod", Name: "acme-aurora-prod", Type: "dbc", RawStruct: cluster}
}

// rel2RDSFake serves DescribeDBSubnetGroups for the dbc half of row 1.
type rel2RDSFake struct {
	awsclient.RDSAPI
	groups []rdstypes.DBSubnetGroup
	err    error
}

func (f *rel2RDSFake) DescribeDBSubnetGroups(
	_ context.Context, _ *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options),
) (*rds.DescribeDBSubnetGroupsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.groups}, nil
}

var _ awsclient.RDSAPI = (*rel2RDSFake)(nil)

// TestRel2DbcSubnetNoGroupNameIsAProvenZero pins the dbc twin of the first two
// answers: a cluster naming no subnet group has no subnets, and the checker
// knows that without calling anything.
func TestRel2DbcSubnetNoGroupNameIsAProvenZero(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: &rel2RDSFake{}}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2DbcCluster(""), resource.ResourceCache{})

	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want Resolved: a cluster naming no subnet group has no subnets", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// TestRel2DbcSubnetCallFailureIsAnError pins the dbc twin of the third answer.
func TestRel2DbcSubnetCallFailureIsAnError(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: &rel2RDSFake{
		err: errors.New("AccessDenied: rds:DescribeDBSubnetGroups"),
	}}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2DbcCluster("acme-aurora-subnet-group"), resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want Error", result.State())
	}
	if result.Err() == nil {
		t.Errorf("Err = nil; the panel has nothing to show the operator")
	}
}

// TestRel2DbcSubnetResolvesWhatItFinds is the counterpart the other two must
// not disturb: a cluster whose subnet group exists still resolves its subnets.
func TestRel2DbcSubnetResolvesWhatItFinds(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: &rel2RDSFake{groups: []rdstypes.DBSubnetGroup{{
		DBSubnetGroupName: aws.String("acme-aurora-subnet-group"),
		VpcId:             aws.String("vpc-prod-main"),
		Subnets: []rdstypes.Subnet{
			{SubnetIdentifier: aws.String("subnet-prod-a")},
			{SubnetIdentifier: aws.String("subnet-prod-b")},
		},
	}}}}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, rel2DbcCluster("acme-aurora-subnet-group"), resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
}

// --- row 2: a memberless group's four pivots ---------------------------------

// TestRel2MemberlessRedisGroupResolvesZeroEverywhere pins the four pivots that
// hang off the member cluster. With no member there is nothing to read and
// nothing to guess, so each renders a count of zero rather than a bare label:
// a row with no count tells the operator neither that there is none nor that
// we could not tell.
func TestRel2MemberlessRedisGroupResolvesZeroEverywhere(t *testing.T) {
	clients := &awsclient.ServiceClients{ElastiCache: &mockElastiCacheFullAPI{}}
	cache := rel2RedisSubnetCache()
	cache["sg"] = resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "sg-prod-redis"}}}
	cache["sns"] = resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "acme-alerts"}}}
	cache["vpc"] = resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "vpc-prod-main"}}}

	for _, target := range []string{"sg", "sns", "subnet", "vpc"} {
		t.Run(target, func(t *testing.T) {
			checker := redisCheckerByTarget(t, target)
			result := checker(context.Background(), clients, rel2RedisGroup(), cache)

			if result.State() != domain.RelatedResolved {
				t.Errorf("State = %v, want Resolved", result.State())
			}
			if result.Count() != 0 {
				t.Errorf("Count = %d, want 0", result.Count())
			}
		})
	}
}

// --- the fourth answer: a row that was never enriched ------------------------

// TestRel2WarmCacheRowIsUnknownNotAnError pins the state a row restored from
// the on-disk cache must produce. That cache deliberately carries no RawStruct
// (core/cache/cache.go), and the detail workload hands the row to the related
// checkers as it stands, before enrichment lands. Nothing failed on that path
// and nothing was read, so the panel owes the user a "?", not a red error
// claiming the details "are not loaded" — which is a description of Unknown,
// not of a failure.
func TestRel2WarmCacheRowIsUnknownNotAnError(t *testing.T) {
	redisClients := &awsclient.ServiceClients{ElastiCache: &mockElastiCacheFullAPI{}}
	dbcClients := &awsclient.ServiceClients{RDS: &rel2RDSFake{}}

	cases := []struct {
		name    string
		checker func(*testing.T, string) resource.RelatedChecker
		clients any
		typ     string
		targets []string
	}{
		{"redis", redisCheckerByTarget, redisClients, "redis", []string{"sg", "sns", "subnet", "vpc"}},
		{"dbc", dbcCheckerByTarget, dbcClients, "dbc", []string{"subnet", "vpc"}},
	}

	for _, tc := range cases {
		for _, target := range tc.targets {
			t.Run(tc.name+"/"+target, func(t *testing.T) {
				// A warm-cache replay row: identity and Fields only.
				row := resource.Resource{
					ID:     "acme-warm-cache-row",
					Name:   "acme-warm-cache-row",
					Type:   tc.typ,
					Fields: map[string]string{"status": "available"},
				}
				result := tc.checker(t, target)(context.Background(), tc.clients, row, rel2RedisSubnetCache())
				if result.State() != domain.RelatedUnknown {
					t.Errorf("State = %v (err %q), want RelatedUnknown: the row was never enriched, so nothing was read and nothing failed",
						result.State(), result.Err())
				}
			})
		}
	}
}

// --- row 2 on the rendered panel --------------------------------------------

// TestRel2MemberlessDemoGroupRendersFourZeros pins row 2 where the operator
// reads it: the panel. "dev-feature-redis" is a demo replication group AWS
// reports with no member clusters, and the four pivots that hang off the
// member must each render a count of zero. A row with no count at all tells
// the operator neither that there is none nor that we could not tell, which is
// the ambiguity this row exists to remove.
func TestRel2MemberlessDemoGroupRendersFourZeros(t *testing.T) {
	rel2AssertRenderedRedisZeros(t, "dev-feature-redis", "sg", "sns", "subnet", "vpc")
}

// TestRel2NoSubnetGroupDemoGroupRendersZeros pins the demo witness on the
// panel: "legacy-redis-classic" has a member cluster that names no subnet
// group, so the subnet and vpc pivots have nothing to reach. That is a fact
// about the group, and the panel must state it as a zero rather than a "?"
// that would also mean "the call failed".
func TestRel2NoSubnetGroupDemoGroupRendersZeros(t *testing.T) {
	rel2AssertRenderedRedisZeros(t, fixtures.RedisNoSubnetGroupID, "subnet", "vpc")
}

// rel2AssertRenderedRedisZeros drives every registered redis checker against a
// demo replication group and asserts the named pivots render a resolved zero
// in the right column the Controller builds.
func rel2AssertRenderedRedisZeros(t *testing.T, groupID string, targets ...string) {
	t.Helper()
	rows := rel2DemoList(t, "redis")
	var group resource.Resource
	for _, r := range rows {
		if r.ID == groupID {
			group = r
		}
	}
	if group.ID == "" {
		t.Fatalf("%s is not in the demo redis list; the row has no witness", groupID)
	}

	clients := demo.NewServiceClients()
	cache := resource.ResourceCache{}
	for _, target := range []string{"sg", "sns", "subnet", "vpc"} {
		cache[target] = resource.ResourceCacheEntry{Resources: rel2DemoList(t, target)}
	}

	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "redis", ResourceID: group.ID},
	}})
	c.EnsureDetailState(group, "redis")
	c.InitDetailRelatedRows("redis")

	want := make(map[string]bool, len(targets))
	for _, target := range targets {
		want[target] = true
	}
	for _, def := range resource.GetRelated("redis") {
		if def.Checker == nil {
			continue
		}
		result := def.Checker(context.Background(), clients, group, cache)
		errMsg := ""
		if result.Err() != nil {
			errMsg = result.Err().Error()
		}
		c.ApplyDetailRelatedResultForResource("redis", group.ID, def.DisplayName, def.TargetType,
			result.EffectiveState(), result.Count(), false, errMsg, result.Truncated(),
			result.ResourceIDs(), result.FetchFilter())
	}

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	seen := map[string]bool{}
	for _, block := range body.Related {
		if !want[block.TargetType] {
			continue
		}
		seen[block.TargetType] = true
		if block.State != domain.RelatedResolved {
			t.Errorf("%s (%s): State = %v, want Resolved", block.Name, block.TargetType, block.State)
		}
		if block.Count != 0 {
			t.Errorf("%s (%s): Count = %d, want 0", block.Name, block.TargetType, block.Count)
		}
	}
	for target := range want {
		if !seen[target] {
			t.Errorf("no rendered related row for %s on %s", target, groupID)
		}
	}
}
