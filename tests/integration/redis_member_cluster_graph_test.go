//go:build integration

package integration

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The redis related panel reads security groups, subnet group and VPC off the
// member cluster named in ReplicationGroup.MemberClusters[0], not off the
// replication group. A demo row that names a member cluster the fake does not
// return makes all three pivots answer "unknown" — the operator reads "?" for
// a fact the fixture set knows. Unknown is the honest answer for a row with no
// members at all, so only rows that name one are held to resolving.
func TestDemoRedisRowsResolveTheMemberClusterTheyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	clients := demo.NewServiceClients()

	pf := resource.GetPaginatedFetcher("redis")
	if pf == nil {
		t.Fatal("redis has no paginated fetcher")
	}
	page, err := pf(ctx, clients, "")
	if err != nil && len(page.Resources) == 0 {
		t.Fatalf("redis fetcher failed: %v", err)
	}
	if len(page.Resources) == 0 {
		t.Fatal("redis fetcher returned no rows; nothing to check")
	}

	memberDriven := map[string]bool{"Security Groups": true, "Subnets": true, "VPC": true}
	defs := resource.GetRelated("redis")
	cache := make(resource.ResourceCache)
	for _, def := range defs {
		if !memberDriven[def.DisplayName] {
			continue
		}
		if _, ok := cache[def.TargetType]; ok {
			continue
		}
		tpf := resource.GetPaginatedFetcher(def.TargetType)
		if tpf == nil {
			continue
		}
		result, terr := tpf(ctx, clients, "")
		if terr != nil && len(result.Resources) == 0 {
			t.Fatalf("related target fetcher for redis -> %s failed: %v", def.TargetType, terr)
		}
		cache[def.TargetType] = resource.ResourceCacheEntry{
			Resources:   result.Resources,
			IsTruncated: result.Pagination != nil && result.Pagination.IsTruncated,
			Pagination:  result.Pagination,
		}
	}

	var orphans []string
	for _, res := range page.Resources {
		rg, ok := res.RawStruct.(elasticachetypes.ReplicationGroup)
		if !ok {
			t.Fatalf("redis row %s carries %T, expected elasticachetypes.ReplicationGroup", res.ID, res.RawStruct)
		}
		if len(rg.MemberClusters) == 0 {
			continue
		}
		for _, def := range defs {
			if !memberDriven[def.DisplayName] {
				continue
			}
			got := def.Checker(ctx, clients, res, cache)
			if got.Err() != nil {
				t.Fatalf("redis %s: %q errored: %v", res.ID, def.DisplayName, got.Err())
			}
			if got.State() != domain.RelatedResolved {
				orphans = append(orphans, res.ID+" names member "+rg.MemberClusters[0]+": "+def.DisplayName+" answered "+got.State().String())
			}
		}
	}
	if len(orphans) > 0 {
		sort.Strings(orphans)
		t.Errorf("%d member-cluster pivot(s) unresolved because the demo fake answers no such cache cluster:\n  %s",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}
