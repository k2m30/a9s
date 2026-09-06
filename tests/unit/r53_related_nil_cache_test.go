package unit_test

// r53_related_nil_cache_test.go — a hosted zone must not invent the resources
// its aliases point at.
//
// Found by the live walk against an account with no load balancers: two zones
// rendered "Load Balancers 2, resolved". The alias records are real, so the
// checker knows a target should exist; what it does not know is which resource
// it is, because the target list came back nil. Reporting the alias DNS name
// as a resolved resource ID turns "I could not look it up" into "here are two
// of them", and the row is then navigable to rows that do not exist.
//
// Nil reaches the checker two ways, and neither means "there are N of these":
// a cache entry whose Resources slice is nil (the app seeds it straight from a
// fetcher that returned nothing), and a cache miss with no registered fetcher.
// The first is a proven empty list, the second is unknown. Neither is a count.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// nilCacheR53Fake serves one page of record sets and one page of query-logging
// configs, which is everything the five alias checkers read from Route 53.
type nilCacheR53Fake struct {
	records    []r53types.ResourceRecordSet
	logConfigs []r53types.QueryLoggingConfig
}

func (f *nilCacheR53Fake) ListHostedZones(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{}, nil
}

func (f *nilCacheR53Fake) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.records}, nil
}

func (f *nilCacheR53Fake) GetHostedZone(_ context.Context, _ *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	return &route53.GetHostedZoneOutput{}, nil
}

func (f *nilCacheR53Fake) ListQueryLoggingConfigs(_ context.Context, _ *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	return &route53.ListQueryLoggingConfigsOutput{QueryLoggingConfigs: f.logConfigs}, nil
}

func (f *nilCacheR53Fake) ListHostedZonesByVPC(_ context.Context, _ *route53.ListHostedZonesByVPCInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error) {
	return &route53.ListHostedZonesByVPCOutput{}, nil
}

var _ awsclient.Route53API = (*nilCacheR53Fake)(nil)

// aliasRecord is the shape Route 53 returns for an A-record alias.
func aliasRecord(name, dns string) r53types.ResourceRecordSet {
	return r53types.ResourceRecordSet{
		Name:        aws.String(name),
		Type:        r53types.RRTypeA,
		AliasTarget: &r53types.AliasTarget{DNSName: aws.String(dns), HostedZoneId: aws.String("Z2FDTNDATAQYW2")},
	}
}

// nilCacheCase is one alias-following checker: the zone content that makes it
// look for a target, and a target-type resource that would never match it.
type nilCacheCase struct {
	target string
	// aliases are two records pointing at targets of this type, so a checker
	// that falls back to DNS names reports 2 rather than 0.
	aliases []r53types.ResourceRecordSet
	// logConfigs is non-nil only for the logs checker, which reads its targets
	// from ListQueryLoggingConfigs rather than from alias records.
	logConfigs []r53types.QueryLoggingConfig
	// unrelated is a row of the target type that matches none of the aliases,
	// used for the truncated-page case.
	unrelated resource.Resource
}

func nilCacheCases() []nilCacheCase {
	return []nilCacheCase{
		{
			target: "elb",
			aliases: []r53types.ResourceRecordSet{
				aliasRecord("www.acme.example.", "dualstack.acme-web-1234567890.us-east-1.elb.amazonaws.com."),
				aliasRecord("api.acme.example.", "dualstack.acme-api-0987654321.us-east-1.elb.amazonaws.com."),
			},
			unrelated: resource.Resource{ID: "acme-other", Fields: map[string]string{
				"dns_name": "acme-other-5555555555.us-east-1.elb.amazonaws.com",
			}},
		},
		{
			target: "cf",
			aliases: []r53types.ResourceRecordSet{
				aliasRecord("cdn.acme.example.", "d111111abcdef8.cloudfront.net."),
				aliasRecord("img.acme.example.", "d222222abcdef8.cloudfront.net."),
			},
			unrelated: resource.Resource{ID: "EOTHER1234567", Fields: map[string]string{
				"domain_name": "d999999abcdef8.cloudfront.net",
			}},
		},
		{
			target: "apigw",
			aliases: []r53types.ResourceRecordSet{
				aliasRecord("v1.acme.example.", "abcd1234ef.execute-api.us-east-1.amazonaws.com."),
				aliasRecord("v2.acme.example.", "wxyz5678gh.execute-api.us-east-1.amazonaws.com."),
			},
			unrelated: resource.Resource{ID: "other9999", Fields: map[string]string{}},
		},
		{
			target: "s3",
			aliases: []r53types.ResourceRecordSet{
				aliasRecord("site.acme.example.", "acme-site.s3-website-us-east-1.amazonaws.com."),
				aliasRecord("docs.acme.example.", "acme-docs.s3-website-us-east-1.amazonaws.com."),
			},
			unrelated: resource.Resource{ID: "acme-other-bucket", Fields: map[string]string{}},
		},
		{
			target: "logs",
			logConfigs: []r53types.QueryLoggingConfig{
				{
					Id:                        aws.String("qlc-1"),
					HostedZoneId:              aws.String("Z1EXAMPLE"),
					CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/route53/acme.example:*"),
				},
				{
					Id:                        aws.String("qlc-2"),
					HostedZoneId:              aws.String("Z1EXAMPLE"),
					CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/route53/other.example:*"),
				},
			},
			unrelated: resource.Resource{ID: "/aws/lambda/acme-unrelated", Fields: map[string]string{}},
		},
	}
}

func (c nilCacheCase) clients() *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Route53: &nilCacheR53Fake{records: c.aliases, logConfigs: c.logConfigs}}
}

// r53Zone is the source row every case checks from.
func r53Zone() resource.Resource {
	return resource.Resource{ID: "Z1EXAMPLE", Name: "acme.example.", Fields: map[string]string{"name": "acme.example."}}
}

// assertNoDNSNameIDs pins the invariant underneath all three cases: whatever a
// checker reports, every ID it returns must be an ID of the target type. A DNS
// name is not one, and the related panel would navigate to nothing.
func assertNoDNSNameIDs(t *testing.T, target string, result domain.RelatedCheckResult) {
	t.Helper()
	for _, id := range result.ResourceIDs() {
		if strings.Contains(id, ".amazonaws.com") || strings.Contains(id, ".cloudfront.net") {
			t.Errorf("%s: returned %q as a resource ID — that is an alias DNS name, not a %s ID; "+
				"the panel would offer a row that resolves to nothing", target, id, target)
		}
	}
}

// TestR53Related_EmptyTargetCache_ResolvesZero is the walk's own case: the
// account holds zero resources of the target type, so the cache entry is
// present and empty. That is a complete answer, and the answer is none.
func TestR53Related_EmptyTargetCache_ResolvesZero(t *testing.T) {
	for _, tc := range nilCacheCases() {
		t.Run(tc.target, func(t *testing.T) {
			// The app seeds the entry straight from a fetcher that found
			// nothing, so Resources is nil rather than an empty slice. Present
			// with nothing in it is still a proven empty list.
			cache := resource.ResourceCache{tc.target: resource.ResourceCacheEntry{Resources: nil}}

			result := r53CheckerByTarget(t, tc.target)(context.Background(), tc.clients(), r53Zone(), cache)

			assertNoDNSNameIDs(t, tc.target, result)
			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: an empty cache entry is a complete answer", got)
			}
			if result.Count() != 0 {
				t.Errorf("Count = %d, want 0 — the account holds no %s, so the zone's aliases resolve to none; got IDs %v",
					result.Count(), tc.target, result.ResourceIDs())
			}
		})
	}
}

// TestR53Related_NoCacheNoFetcher_Unknown is the other producer of a nil list:
// nothing to read and nothing to call. The checker cannot know, and saying so
// is the honest answer — the same one acm_related.go already gives.
func TestR53Related_NoCacheNoFetcher_Unknown(t *testing.T) {
	for _, tc := range nilCacheCases() {
		t.Run(tc.target, func(t *testing.T) {
			restore := resource.GetPaginatedFetcher(tc.target)
			resource.SetPaginatedForTest(tc.target, nil)
			t.Cleanup(func() { resource.SetPaginatedForTest(tc.target, restore) })

			result := r53CheckerByTarget(t, tc.target)(context.Background(), tc.clients(), r53Zone(), resource.ResourceCache{})

			assertNoDNSNameIDs(t, tc.target, result)
			if got := result.EffectiveState(); got != domain.RelatedUnknown {
				t.Errorf("state = %v with no cache entry and no fetcher, want RelatedUnknown; Count=%d IDs=%v",
					got, result.Count(), result.ResourceIDs())
			}
		})
	}
}

// TestR53Related_TruncatedTargetPageNoMatch_Truncated is INVERTED (was: the
// same case asserted Unknown). Row 16 settled it the other way: a list that WAS
// read is evidence, so nothing matching in it is a real zero so far, and the
// truncation flag is what says a later page may add to it. Unknown is reserved
// for a list that was never read at all, which the test above covers.
func TestR53Related_TruncatedTargetPageNoMatch_Truncated(t *testing.T) {
	for _, tc := range nilCacheCases() {
		t.Run(tc.target, func(t *testing.T) {
			cache := resource.ResourceCache{tc.target: resource.ResourceCacheEntry{
				Resources:   []resource.Resource{tc.unrelated},
				IsTruncated: true,
			}}

			result := r53CheckerByTarget(t, tc.target)(context.Background(), tc.clients(), r53Zone(), cache)

			assertNoDNSNameIDs(t, tc.target, result)
			if len(result.ResourceIDs()) > 0 {
				return // a match on the first page is authoritative; nothing to prove here
			}
			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Errorf("state = %v after a truncated first page matched nothing, want RelatedResolved", got)
			}
			if !result.Truncated() {
				t.Error("Truncated = false, want true — a later page may still match")
			}
		})
	}
}

// TestR53Related_DemoBench_ZoneWithNoMatchingLoadBalancer is the demo-bench
// witness for the defect the live walk found. acme-corp.com. aliases
// api.acme-corp.com. at prod-api-alb-1234567890, a name no demo load balancer
// carries, so its Load Balancers panel must read a resolved zero; before the
// fix it read 1 with that hostname as the row's ID. staging.acme-corp.com.
// aliases a load balancer that does exist, and is here so a fix that simply
// zeroed the panel would fail too.
func TestR53Related_DemoBench_ZoneWithNoMatchingLoadBalancer(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	elbRows := byType["elb"]
	if len(elbRows) == 0 {
		t.Fatal("no demo elb fixtures — the panel cannot be witnessed")
	}
	cache := resource.ResourceCache{"elb": resource.ResourceCacheEntry{Resources: elbRows}}
	clients := demo.NewServiceClients()
	checker := r53CheckerByTarget(t, "elb")

	// The walk ran against an account holding no load balancers at all, which
	// is the cache entry the app seeds from a fetcher that found none. That is
	// the only shape in which the old code reached its guessing branch, so the
	// witness needs it: with the full demo list present the branch never runs
	// and the case would pass either way.
	emptyELB := resource.ResourceCache{"elb": resource.ResourceCacheEntry{Resources: nil}}

	for _, tc := range []struct {
		zoneID string
		name   string
		cache  resource.ResourceCache
		want   int
	}{
		{"/hostedzone/Z0123456789ABCDEFGHIJ", "acme-corp.com. (account with no load balancers)", emptyELB, 0},
		{"/hostedzone/Z0123456789ABCDEFGHIJ", "acme-corp.com.", cache, 0},
		{"/hostedzone/Z2345678901ABCDEFGHIJ", "staging.acme-corp.com.", cache, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zone := resource.Resource{ID: tc.zoneID, Name: tc.name, Fields: map[string]string{"name": tc.name}}
			result := checker(context.Background(), clients, zone, tc.cache)

			assertNoDNSNameIDs(t, "elb", result)
			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the demo elb list is complete", got)
			}
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d; IDs = %v", result.Count(), tc.want, result.ResourceIDs())
			}
		})
	}
}
