package unit_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func copyCache(c resource.ResourceCache) resource.ResourceCache {
	out := make(resource.ResourceCache, len(c))
	for k, v := range c {
		out[k] = v
	}
	return out
}

func cacheWithTruncatedEntry(c resource.ResourceCache, key string) resource.ResourceCache {
	out := copyCache(c)
	entry := out[key]
	entry.IsTruncated = true
	out[key] = entry
	return out
}

func relatedDisplayName(t *testing.T, source, target string) string {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target {
			return def.DisplayName
		}
	}
	t.Fatalf("no %s related def for target %q", source, target)
	return ""
}

// A pivot that reaches its targets through an intermediate list answers for
// as much as it read: if the intermediate page was partial, a target reachable
// only through an unread row is missing from the count, so the count is a
// lower bound.
func TestRelatedTruncation_FirstHopTruncationReachesTheCount(t *testing.T) {
	b := newRefBench(t)
	cases := []struct {
		source, target, row, firstHop string
		wantIDs                       []string
	}{
		{"ecs-svc", "elb", "acme-services/api-gateway", "tg", []string{"acme-prod-web"}},
		{"ecs-task", "sg", "a1b2c3d4e5f6a1b2c3d4e5f6", "eni", []string{"sg-0aaa111111111111a"}},
		{"subnet", "efs", "subnet-0efs0prod00000a", "eni", []string{"fs-0prod1234abcd5678"}},
	}
	for _, tc := range cases {
		src := b.row(t, tc.source, tc.row)
		check := refChecker(t, tc.source, tc.target)
		name := relatedDisplayName(t, tc.source, tc.target)

		t.Run(tc.source+"/"+tc.target+"/both hops complete", func(t *testing.T) {
			r := check(context.Background(), refClients(), src, b.cache)
			if got := sortedIDs(r); fmt.Sprint(got) != fmt.Sprint(tc.wantIDs) {
				t.Fatalf("ids = %v, want %v", got, tc.wantIDs)
			}
			if r.Truncated() || r.Coverage() != resource.CoverageComplete {
				t.Errorf("truncated = %v, coverage = %v, want an exact count", r.Truncated(), r.Coverage())
			}
			blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{name: r})
			assertRelatedRow(t, blocks, rendered, name, fmt.Sprintf("(%d)", len(tc.wantIDs)), true)
		})

		t.Run(tc.source+"/"+tc.target+"/"+tc.firstHop+" page partial", func(t *testing.T) {
			r := check(context.Background(), refClients(), src, cacheWithTruncatedEntry(b.cache, tc.firstHop))
			if got := sortedIDs(r); fmt.Sprint(got) != fmt.Sprint(tc.wantIDs) {
				t.Fatalf("ids = %v, want %v", got, tc.wantIDs)
			}
			if !r.Truncated() || r.Coverage() != resource.CoveragePartial {
				t.Errorf("truncated = %v, coverage = %v, want a lower bound: the %s list was read in part",
					r.Truncated(), r.Coverage(), tc.firstHop)
			}
			blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{name: r})
			assertRelatedRow(t, blocks, rendered, name, fmt.Sprintf("(%d+)", len(tc.wantIDs)), true)
		})
	}
}

// A target list fetched for one pivot and a target list fetched for the list
// view are the same list: rows returned beside a per-item failure are a
// proven subset either way, so both reads report the same completeness.
func TestRelatedTruncation_PrefetchedTargetListAnswersLikeTheFetch(t *testing.T) {
	const target = "sqs"
	queue := resource.Resource{ID: "order-processing-queue", Name: "order-processing-queue", Type: target}
	denied := errors.New("1 of 2 queues could not be read: AccessDenied")

	cases := []struct {
		name string
		rows []resource.Resource
		err  error
	}{
		{"rows beside a composite error", []resource.Resource{queue}, denied},
		{"a row whose details could not be read", []resource.Resource{queue, awsclient.DegradedDetails(target, "payments-dlq", denied)}, nil},
	}
	probe := func(ctx context.Context, clients any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		list, truncated, err := awsclient.FetchRelatedTarget(ctx, clients, cache, target)
		if err != nil {
			return resource.ErrorRelated(target, err)
		}
		ids := make([]string, 0, len(list))
		for _, row := range list {
			ids = append(ids, row.ID)
		}
		return resource.KnownRelated(target, ids, truncated)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource.SetPaginatedForTest(target, func(context.Context, any, string) (resource.FetchResult, error) {
				return resource.FetchResult{Resources: tc.rows}, tc.err
			})
			t.Cleanup(func() { resource.CleanupPaginatedForTest(target) })

			def := resource.RelatedDef{TargetType: target, DisplayName: "Queues", Checker: probe, NeedsTargetCache: true, Truncated: true}
			op := runtime.DetailOperation{
				ResourceType: "lambda",
				Resource:     resource.Resource{ID: "process-orders", Name: "process-orders", Type: "lambda"},
				Clients:      refClients(),
			}
			prefetched := runtime.RunRelatedDef(context.Background(), op, resource.ResourceCache{}, map[string]struct{}{}, def).Result
			fetched := probe(context.Background(), refClients(), resource.Resource{}, resource.ResourceCache{})

			if !fetched.Truncated() || fetched.Coverage() != resource.CoveragePartial {
				t.Fatalf("the fetch path reports truncated = %v, coverage = %v, want a lower bound",
					fetched.Truncated(), fetched.Coverage())
			}
			if prefetched.Truncated() != fetched.Truncated() || prefetched.Coverage() != fetched.Coverage() {
				t.Errorf("prefetched = (truncated %v, coverage %v), fetched = (truncated %v, coverage %v); one list, one answer",
					prefetched.Truncated(), prefetched.Coverage(), fetched.Truncated(), fetched.Coverage())
			}
		})
	}
}

// A scan that cannot read a row of the target list may have skipped the
// match: a degraded row carries no SDK struct at all, and neither does a row
// of a different type, so a count over the rest is a lower bound.
func TestRelatedTruncation_UnreadableTargetRowsMakeTheCountALowerBound(t *testing.T) {
	b := newRefBench(t)
	src := b.row(t, "lt", "lt-0eks111111111111a")
	check := refChecker(t, "lt", "ng")
	wantIDs := []string{"acme-prod/general-pool"}

	ngRows := func() []resource.Resource { return append([]resource.Resource(nil), b.byType["ng"]...) }
	// The demo account answers DescribeNodegroup for some node groups only;
	// the rows it cannot describe carry no SDK struct.
	readableNGRows := func() []resource.Resource {
		var out []resource.Resource
		for _, r := range b.byType["ng"] {
			if r.Fields[awsclient.DegradedFindingField] == "" {
				out = append(out, r)
			}
		}
		return out
	}
	withNG := func(rows []resource.Resource) resource.ResourceCache {
		c := copyCache(b.cache)
		c["ng"] = resource.ResourceCacheEntry{Resources: rows}
		return c
	}
	otherType := func(rows []resource.Resource) []resource.Resource {
		for i, r := range rows {
			if r.ID != wantIDs[0] {
				rows[i].RawStruct = ekstypes.Cluster{Name: aws.String(r.ID), Status: ekstypes.ClusterStatusActive}
				return rows
			}
		}
		t.Fatal("demo bench has no second ng row")
		return nil
	}

	cases := []struct {
		name      string
		rows      []resource.Resource
		wantTrunc bool
		wantCov   resource.RelatedCoverage
	}{
		{"every row readable", readableNGRows(), false, resource.CoverageComplete},
		{"a row whose details could not be read", ngRows(), true, resource.CoveragePartial},
		{"a row that is not a node group", otherType(readableNGRows()), true, resource.CoveragePartial},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := check(context.Background(), refClients(), src, withNG(tc.rows))
			if got := sortedIDs(r); fmt.Sprint(got) != fmt.Sprint(wantIDs) {
				t.Fatalf("ids = %v, want %v", got, wantIDs)
			}
			if r.Truncated() != tc.wantTrunc || r.Coverage() != tc.wantCov {
				t.Errorf("truncated = %v, coverage = %v, want (%v, %v)", r.Truncated(), r.Coverage(), tc.wantTrunc, tc.wantCov)
			}
		})
	}
}

// foreignSLRIAM answers GetRole with a role of another account, which this
// account's role list cannot hold.
type foreignSLRIAM struct {
	awsclient.IAMAPI
}

func (f foreignSLRIAM) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	return &iam.GetRoleOutput{Role: &iamtypes.Role{
		RoleName: in.RoleName,
		Arn:      aws.String("arn:aws:iam::210987654321:role/aws-service-role/transitgateway.amazonaws.com/" + aws.ToString(in.RoleName)),
		RoleId:   aws.String("AROAEXAMPLEID0000002"),
		Path:     aws.String("/aws-service-role/transitgateway.amazonaws.com/"),
	}}, nil
}

// A pivot that matches by a shared property rather than a recorded link
// offers candidates, not a count. Reading less of the list makes the
// candidate set smaller; it does not turn it into a number.
func TestRelatedTruncation_HeuristicPivotStaysHeuristicWhenScanIsPartial(t *testing.T) {
	b := newRefBench(t)
	cases := []struct {
		source, target, row string
		result              func(t *testing.T, src resource.Resource) resource.RelatedCheckResult
	}{
		{"ec2", "tg", "i-0a1b2c3d4e5f60001", func(t *testing.T, src resource.Resource) resource.RelatedCheckResult {
			return refChecker(t, "ec2", "tg")(context.Background(), refClients(), src, cacheWithTruncatedEntry(b.cache, "tg"))
		}},
		{"secrets", "codeartifact", "prod/codeartifact/npm-publish-token", func(t *testing.T, src resource.Resource) resource.RelatedCheckResult {
			return refChecker(t, "secrets", "codeartifact")(context.Background(), refClients(), src, cacheWithTruncatedEntry(b.cache, "codeartifact"))
		}},
		{"tgw", "role", "tgw-0aaa111111111111a", func(t *testing.T, src resource.Resource) resource.RelatedCheckResult {
			c := refClients()
			c.IAM = foreignSLRIAM{IAMAPI: c.IAM}
			return refChecker(t, "tgw", "role")(context.Background(), c, src, b.cache)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.source+"/"+tc.target, func(t *testing.T) {
			src := b.row(t, tc.source, tc.row)
			r := tc.result(t, src)
			if r.Coverage() != resource.CoverageHeuristic {
				t.Errorf("coverage = %v, want CoverageHeuristic", r.Coverage())
			}
			name := relatedDisplayName(t, tc.source, tc.target)
			blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{name: r})
			assertRelatedRow(t, blocks, rendered, name, "", true)
		})
	}
}

// RelatedDef.Truncated is the per-resource docs' "Truncated?" column: a pivot
// that can answer with a lower bound must say so. A heuristic answer carries
// no number and is not described by that column.
func TestRelatedTruncation_EveryLowerBoundPivotDeclaresIt(t *testing.T) {
	b := newRefBench(t)
	partialCache := copyCache(b.cache)
	for key, entry := range partialCache {
		entry.IsTruncated = true
		partialCache[key] = entry
	}
	clients := refClients()

	var undeclared []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			if def.Truncated || def.Checker == nil {
				continue
			}
			for _, row := range b.byType[td.ShortName] {
				hit := false
				for _, cache := range []resource.ResourceCache{b.cache, partialCache} {
					r := def.Checker(context.Background(), clients, row, cache)
					if r.Coverage() == resource.CoverageHeuristic {
						continue
					}
					if r.Truncated() || r.Coverage() == resource.CoveragePartial {
						hit = true
						break
					}
				}
				if hit {
					undeclared = append(undeclared, fmt.Sprintf("%s → %s (%s), first %s", td.ShortName, def.TargetType, def.DisplayName, row.ID))
					break
				}
			}
		}
	}
	sort.Strings(undeclared)
	for _, v := range undeclared {
		t.Errorf("returns a lower bound with Truncated: false — %s", v)
	}
}

// relatedResourcesFor answers with a nil list when nothing was read at all.
// A count over a list nobody read is a guess, so the row is unknown.
func TestRelatedTruncation_RedisWithNoTargetListIsUnknown(t *testing.T) {
	b := newRefBench(t)
	src := b.row(t, "redis", "warn-redis-at-rest-off")
	for _, target := range []string{"sg", "subnet"} {
		t.Run(target, func(t *testing.T) {
			resource.SetPaginatedForTest(target, nil)
			t.Cleanup(func() { resource.CleanupPaginatedForTest(target) })
			cache := copyCache(b.cache)
			delete(cache, target)

			r := refChecker(t, "redis", target)(context.Background(), refClients(), src, cache)
			if r.State() != domain.RelatedUnknown {
				t.Errorf("state = %v (count %d, coverage %v), want unknown: the %s list was never read",
					r.State(), r.Count(), r.Coverage(), target)
			}
			if r.Coverage() == resource.CoverageComplete {
				t.Errorf("coverage = CoverageComplete for a %s list that was never read", target)
			}
		})
	}
}
