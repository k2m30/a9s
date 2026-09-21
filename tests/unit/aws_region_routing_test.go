package unit_test

// Which region each read is issued against, for data that does not live in
// the session's region: CloudFront-scope WAF, CloudFront's certificates and
// alarms, and Route 53 query logs live in us-east-1; a bucket lives in its own
// region; a trail's log group lives in the trail's home region. Read from the
// session region, each is reported missing.

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func rwCodes(fs []domain.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f.Code))
	}
	sort.Strings(out)
	return out
}

func rwRequireResolved(t *testing.T, got resource.RelatedCheckResult, want ...string) {
	t.Helper()
	if got.EffectiveState() != domain.RelatedResolved || got.Truncated() {
		t.Fatalf("state = %v truncated=%v err=%v, want a resolved, exact answer %v", got.EffectiveState(), got.Truncated(), got.Err(), want)
	}
	sort.Strings(want)
	if !slices.Equal(got.ResourceIDs(), want) {
		t.Errorf("ids = %v, want %v", got.ResourceIDs(), want)
	}
}

// ─── CloudFront-scope WAF lives in us-east-1 ────────────────────────

func rwWAFResources(t *testing.T, w *rwWorld, clients *awsclient.ServiceClients) (edge, api resource.Resource) {
	t.Helper()
	acls := rwFetch(t, clients, "waf")
	edge, api = rwByID(t, acls, rwEdgeACLID), rwByID(t, acls, rwAPIACLID)
	if edge.Fields["scope"] != "CLOUDFRONT" || api.Fields["scope"] != "REGIONAL" {
		t.Fatalf("scopes = %q/%q, want CLOUDFRONT/REGIONAL", edge.Fields["scope"], api.Fields["scope"])
	}
	for _, c := range w.callsFor("wafv2", "") {
		if c.Op == "ListWebACLs" && c.Region != "us-east-1" && c.Region != clients.Region {
			t.Errorf("ListWebACLs issued against %s, neither the session region nor us-east-1", c.Region)
		}
	}
	w.resetCalls()
	return edge, api
}

// A CloudFront web ACL is readable only through the us-east-1 endpoint; from
// eu-west-1 its logging, associations and rules are asked of a region where
// it does not exist. The regional ACL in the same pass stays in the session
// region and keeps its own verdict, so the pass can tell the two apart.
func TestRegionRouting_WAFEnrichmentReadsCloudFrontACLInUSEast1(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	edge, api := rwWAFResources(t, w, clients)

	res, err := awsclient.EnrichWAFLogging(context.Background(), clients, []resource.Resource{edge, api}, resource.ResourceCache{})
	if err != nil {
		t.Errorf("EnrichWAFLogging error = %v, want nil", err)
	}

	if reason, ok := res.TruncatedIDs[rwEdgeACLID]; ok {
		t.Errorf("CloudFront ACL marked uninspected (%q); its us-east-1 answers were all available", reason)
	}
	for _, f := range res.Findings[rwEdgeACLID] {
		if f.Code == "waf.no-logging" {
			t.Errorf("CloudFront ACL reported %q; it logs to %s", f.Phrase, rwEdgeLogArn)
		}
	}
	if got := res.FieldUpdates[rwEdgeACLID]["rules_summary"]; got != "1/2 BLOCK" {
		t.Errorf("CloudFront ACL rules_summary = %q, want %q (read from GetWebACL in us-east-1)", got, "1/2 BLOCK")
	}
	w.requireCallsOnlyIn(t, "wafv2", rwEdgeACLID, "us-east-1")

	if got := rwCodes(res.Findings[rwAPIACLID]); !slices.Equal(got, []string{"waf.no-logging"}) {
		t.Errorf("regional ACL findings = %v, want [waf.no-logging]", got)
	}
	if got := res.FieldUpdates[rwAPIACLID]["rules_summary"]; got != "1/3 BLOCK" {
		t.Errorf("regional ACL rules_summary = %q, want %q", got, "1/3 BLOCK")
	}
	w.requireCallsOnlyIn(t, "wafv2", rwAPIACLID, "eu-west-1")
}

// ListResourcesForWebACL does not report CloudFront associations; AWS names
// ListDistributionsByWebACLId as the call that does. A CloudFront ACL bound to
// a distribution is therefore not an orphan, and one bound to none is.
func TestRegionRouting_WAFCloudFrontACLOrphanVerdictComesFromDistributions(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	edge, _ := rwWAFResources(t, w, clients)
	legacy := rwByID(t, rwFetch(t, clients, "waf"), rwLegacyACLID)

	res, err := awsclient.EnrichWAFLogging(context.Background(), clients, []resource.Resource{edge, legacy}, resource.ResourceCache{})
	if err != nil {
		t.Errorf("EnrichWAFLogging error = %v, want nil", err)
	}
	if got := rwCodes(res.Findings[rwEdgeACLID]); len(got) != 0 {
		t.Errorf("CloudFront ACL used by distribution %s: findings = %v, want none", rwDistID, got)
	}
	if got := rwCodes(res.Findings[rwLegacyACLID]); !slices.Equal(got, []string{"waf.orphan"}) {
		t.Errorf("CloudFront ACL no distribution uses: findings = %v, want [waf.orphan]", got)
	}
}

// The related panel of a CloudFront web ACL, opened from eu-west-1, reads the
// ACL's logging and associations from us-east-1. The regional ACL beside it is
// read from eu-west-1.
func TestRegionRouting_WAFRelatedReadsCloudFrontACLInUSEast1(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	edge, api := rwWAFResources(t, w, clients)
	ctx := context.Background()

	cases := []struct {
		name   string
		src    resource.Resource
		target string
		want   []string
	}{
		{"cloudfront ACL logs", edge, "logs", []string{"aws-waf-logs-acme-edge"}},
		{"cloudfront ACL load balancers", edge, "elb", nil},
		{"cloudfront ACL api gateways", edge, "apigw", nil},
		{"cloudfront ACL distributions", edge, "cf", []string{rwDistID}},
		{"regional ACL logs", api, "logs", nil},
		{"regional ACL load balancers", api, "elb", []string{"acme-api-alb"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rwChecker(t, "waf", tc.target)(ctx, clients, tc.src, resource.ResourceCache{})
			rwRequireResolved(t, got, tc.want...)
		})
	}
	w.requireNoCallsIn(t, "wafv2", rwEdgeACLID, "eu-west-1")
	w.requireNoCallsIn(t, "wafv2", rwAPIACLID, "us-east-1")
}

// ─── a bucket's calls go to the bucket's region ─────────────────────

// The S3 tests run from a us-east-1 session against a bucket that lives
// in eu-west-1; S3 answers such calls with PermanentRedirect.
func rwS3Resources(t *testing.T, w *rwWorld, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	buckets := rwFetch(t, clients, "s3")
	w.resetCalls()
	return buckets
}

func TestRegionRouting_S3ListReadsNotificationsInBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	assets := rwByID(t, rwFetch(t, clients, "s3"), rwAssetsBucket)

	if got := assets.Fields["notification_sqs"]; got != rwAssetsQueue {
		t.Errorf("notification_sqs = %q, want %q", got, rwAssetsQueue)
	}
	if got := assets.Fields["notification_truncated"]; got != "" {
		t.Errorf("notification_truncated = %q, want empty: the bucket's own region answered", got)
	}
	w.requireCallsOnlyIn(t, "s3", rwAssetsBucket, "eu-west-1")
}

func TestRegionRouting_S3PostureReadsBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	buckets := rwS3Resources(t, w, clients)
	assets, web := rwByID(t, buckets, rwAssetsBucket), rwByID(t, buckets, rwWebBucket)

	res, err := awsclient.EnrichS3Posture(context.Background(), clients, []resource.Resource{assets, web}, resource.ResourceCache{})
	if err != nil {
		t.Errorf("EnrichS3Posture error = %v, want nil", err)
	}
	if reason, ok := res.TruncatedIDs[rwAssetsBucket]; ok {
		t.Errorf("%s marked uninspected (%q); its region answered every call", rwAssetsBucket, reason)
	}
	if got := rwCodes(res.Findings[rwAssetsBucket]); !slices.Equal(got, []string{"s3.mfa-delete-off", "s3.no-object-lock"}) {
		t.Errorf("%s findings = %v, want [s3.mfa-delete-off s3.no-object-lock]", rwAssetsBucket, got)
	}
	if got := rwCodes(res.Findings[rwWebBucket]); !slices.Equal(got, []string{"s3.no-object-lock", "s3.versioning-off"}) {
		t.Errorf("%s findings = %v, want [s3.no-object-lock s3.versioning-off]", rwWebBucket, got)
	}
	w.requireCallsOnlyIn(t, "s3", rwAssetsBucket, "eu-west-1")
	w.requireCallsOnlyIn(t, "s3", rwWebBucket, "us-east-1")
}

func TestRegionRouting_S3RelatedReadsBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	assets := rwByID(t, rwS3Resources(t, w, clients), rwAssetsBucket)
	ctx := context.Background()

	kms := rwChecker(t, "s3", "kms")(ctx, clients, assets, resource.ResourceCache{})
	if kms.EffectiveState() != domain.RelatedResolved || kms.Truncated() || kms.Count() != 1 {
		t.Errorf("s3 → kms = state %v truncated=%v count=%d, want exactly the bucket's one KMS key", kms.EffectiveState(), kms.Truncated(), kms.Count())
	}
	rwRequireResolved(t, rwChecker(t, "s3", "s3")(ctx, clients, assets, resource.ResourceCache{}), "acme-access-logs-eu")
	w.requireCallsOnlyIn(t, "s3", rwAssetsBucket, "eu-west-1")
}

func TestRegionRouting_S3DetailReadsBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	assets := rwByID(t, rwS3Resources(t, w, clients), rwAssetsBucket)

	enrich := resource.GetDetailEnricher("s3")
	if enrich == nil {
		t.Fatal("no detail enricher registered for s3")
	}
	got, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, assets)
	if err != nil {
		t.Fatalf("s3 detail enrichment: %v", err)
	}
	be, ok := got.RawStruct.(awsclient.BucketEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want awsclient.BucketEnriched", got.RawStruct)
	}
	if be.Policy == nil {
		t.Error("Policy is empty; the bucket has a policy in eu-west-1")
	}
	if len(be.CORSRules) != 1 {
		t.Errorf("CORSRules = %d, want 1", len(be.CORSRules))
	}
	if len(be.LifecycleRules) != 1 || aws.ToString(be.LifecycleRules[0].ID) != "expire-tmp" || be.LifecycleRules[0].Status != s3types.ExpirationStatusEnabled {
		t.Errorf("LifecycleRules = %+v, want the one enabled rule expire-tmp", be.LifecycleRules)
	}
	w.requireCallsOnlyIn(t, "s3", rwAssetsBucket, "eu-west-1")
}

// Entering a bucket lists its objects; the child view knows only the bucket
// name, so the list call must still find the bucket's region.
func TestRegionRouting_S3ObjectsListInBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	child := resource.GetPaginatedChildFetcher("s3_objects")
	if child == nil {
		t.Fatal("no child fetcher registered for s3_objects")
	}
	res, err := child(context.Background(), clients, resource.ParentContext{"bucket": rwAssetsBucket, "prefix": ""}, "")
	if err != nil {
		t.Fatalf("listing %s from a us-east-1 session: %v", rwAssetsBucket, err)
	}
	var ids []string
	for _, r := range res.Resources {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	if !slices.Equal(ids, []string{"css/", "index.html"}) {
		t.Errorf("objects = %v, want [css/ index.html]", ids)
	}
	w.requireCallsOnlyIn(t, "s3", rwAssetsBucket, "eu-west-1")
}

// A trail's log bucket is looked up by name alone; an organisation trail's
// bucket commonly sits in another region than the session.
func TestRegionRouting_TrailLogBucketReadInBucketRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	trails := rwFetch(t, clients, "trail")
	org := rwByID(t, trails, rwOrgTrail)
	w.resetCalls()

	res, err := awsclient.EnrichTrailLogBucket(context.Background(), clients, []resource.Resource{org}, resource.ResourceCache{})
	if err != nil {
		t.Errorf("EnrichTrailLogBucket error = %v, want nil", err)
	}
	if reason, ok := res.TruncatedIDs[rwOrgTrail]; ok {
		t.Errorf("trail marked uninspected (%q); its bucket's region answered", reason)
	}
	if got := rwCodes(res.Findings[rwOrgTrail]); !slices.Equal(got, []string{string(awsclient.CodeTrailLogBucketNoAccessLogging)}) {
		t.Errorf("trail findings = %v, want [%s]", got, awsclient.CodeTrailLogBucketNoAccessLogging)
	}
	w.requireCallsOnlyIn(t, "s3", rwTrailBucket, "us-west-2")
}

// ─── CloudFront's certificate, alarms and Route 53 query logs ────────

func rwDistribution() resource.Resource {
	return resource.Resource{
		ID:   rwDistID,
		Name: "d111111abcdef8.cloudfront.net",
		Fields: map[string]string{
			"distribution_id": rwDistID,
			"domain_name":     "d111111abcdef8.cloudfront.net",
			"status":          "Deployed",
			"enabled":         "true",
			"aliases":         "www.acme-example.com",
		},
		RawStruct: cftypes.DistributionSummary{
			Id:         aws.String(rwDistID),
			ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/" + rwDistID),
			DomainName: aws.String("d111111abcdef8.cloudfront.net"),
			Status:     aws.String("Deployed"),
			Enabled:    aws.Bool(true),
			WebACLId:   aws.String(rwEdgeACLArn),
			Aliases:    &cftypes.Aliases{Quantity: aws.Int32(1), Items: []string{"www.acme-example.com"}},
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn:      aws.String(rwEdgeCertArn),
				SSLSupportMethod:       cftypes.SSLSupportMethodSniOnly,
				MinimumProtocolVersion: cftypes.MinimumProtocolVersionTLSv122021,
			},
		},
	}
}

// The session's own certificate and alarm lists are in the cache, as they are
// once the operator has opened them; neither holds anything CloudFront uses.
func TestRegionRouting_CloudFrontCertificateAndAlarmPivotsReadUSEast1(t *testing.T) {
	for _, session := range []string{"eu-west-1", "us-east-1"} {
		t.Run(session, func(t *testing.T) {
			w := newRegionWorld()
			clients := w.clients(session)
			cache := resource.ResourceCache{
				"acm":   {Resources: rwFetch(t, clients, "acm")},
				"alarm": {Resources: rwFetch(t, clients, "alarm")},
			}
			w.resetCalls()
			ctx := context.Background()

			rwRequireResolved(t, rwChecker(t, "cf", "acm")(ctx, clients, rwDistribution(), cache), rwEdgeCertArn)
			rwRequireResolved(t, rwChecker(t, "cf", "alarm")(ctx, clients, rwDistribution(), cache), rwEdgeAlarm)
			for _, svc := range []string{"acm", "monitoring"} {
				for _, c := range w.callsFor(svc, "") {
					if c.Region != "us-east-1" {
						t.Errorf("%s %s issued against %s; CloudFront's certificates and metrics are only in us-east-1", svc, c.Op, c.Region)
					}
				}
			}
		})
	}
}

// Route 53 writes public-zone query logs only to a us-east-1 log group.
func TestRegionRouting_Route53QueryLogGroupPivotReadsUSEast1(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}
	w.resetCalls()
	zone := resource.Resource{
		ID:     rwZoneID,
		Name:   "acme-example.com.",
		Fields: map[string]string{"zone_id": rwZoneID, "name": "acme-example.com.", "private_zone": "false"},
	}

	rwRequireResolved(t, rwChecker(t, "r53", "logs")(context.Background(), clients, zone, cache), rwZoneLogGroup)
	w.requireCallsOnlyIn(t, "logs", "", "us-east-1")
}

// ─── a trail's log group lives in the trail's home region ───────────

func TestRegionRouting_TrailLogGroupPivotReadsHomeRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	trails := rwFetch(t, clients, "trail")
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}
	ctx := context.Background()

	t.Run("multi-region trail homed in us-west-2", func(t *testing.T) {
		w.resetCalls()
		rwRequireResolved(t, rwChecker(t, "trail", "logs")(ctx, clients, rwByID(t, trails, rwOrgTrail), cache), rwOrgTrailLogGroup)
		w.requireCallsOnlyIn(t, "logs", "", "us-west-2")
	})
	t.Run("trail homed in the session region", func(t *testing.T) {
		w.resetCalls()
		rwRequireResolved(t, rwChecker(t, "trail", "logs")(ctx, clients, rwByID(t, trails, rwEUTrail), cache), rwEUTrailLogGroup)
		for _, c := range w.callsFor("logs", "") {
			if c.Region != "eu-west-1" {
				t.Errorf("logs %s issued against %s for a trail homed in eu-west-1", c.Op, c.Region)
			}
		}
	})
}

// ─── a failed call to another region is unknown, never zero ────────

func rwRequireUnknown(t *testing.T, got resource.RelatedCheckResult) {
	t.Helper()
	switch got.EffectiveState() {
	case domain.RelatedUnknown, domain.RelatedError:
	default:
		t.Errorf("state = %v truncated=%v ids=%v; a refused call in the data's own region must render unknown, not a count", got.EffectiveState(), got.Truncated(), got.ResourceIDs())
	}
}

func TestRegionRouting_FailedCrossRegionCallIsUnknownNotZero(t *testing.T) {
	ctx := context.Background()

	t.Run("cloudfront certificate, us-east-1 ACM refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		cache := resource.ResourceCache{"acm": {Resources: rwFetch(t, clients, "acm")}}
		w.denyAccess("acm", "us-east-1")
		rwRequireUnknown(t, rwChecker(t, "cf", "acm")(ctx, clients, rwDistribution(), cache))
	})
	t.Run("cloudfront alarms, us-east-1 CloudWatch refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		cache := resource.ResourceCache{"alarm": {Resources: rwFetch(t, clients, "alarm")}}
		w.denyAccess("monitoring", "us-east-1")
		rwRequireUnknown(t, rwChecker(t, "cf", "alarm")(ctx, clients, rwDistribution(), cache))
	})
	t.Run("route 53 query log group, us-east-1 logs refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}
		w.denyAccess("logs", "us-east-1")
		zone := resource.Resource{ID: rwZoneID, Name: "acme-example.com.", Fields: map[string]string{"private_zone": "false"}}
		rwRequireUnknown(t, rwChecker(t, "r53", "logs")(ctx, clients, zone, cache))
	})
	t.Run("trail log group, home-region logs refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		trails := rwFetch(t, clients, "trail")
		cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}
		w.denyAccess("logs", "us-west-2")
		rwRequireUnknown(t, rwChecker(t, "trail", "logs")(ctx, clients, rwByID(t, trails, rwOrgTrail), cache))
	})
	t.Run("cloudfront ACL logs, us-east-1 WAF refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		edge, _ := rwWAFResources(t, w, clients)
		w.denyAccess("wafv2", "us-east-1")
		rwRequireUnknown(t, rwChecker(t, "waf", "logs")(ctx, clients, edge, resource.ResourceCache{}))
	})
	t.Run("cloudfront ACL posture, us-east-1 WAF refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("eu-west-1")
		edge, _ := rwWAFResources(t, w, clients)
		w.denyAccess("wafv2", "us-east-1")
		res, _ := awsclient.EnrichWAFLogging(ctx, clients, []resource.Resource{edge}, resource.ResourceCache{}) //nolint:errcheck // the refusal is expected; the row state is asserted
		if got := rwCodes(res.Findings[rwEdgeACLID]); len(got) != 0 {
			t.Errorf("findings = %v, want none: nothing was read", got)
		}
		if _, ok := res.TruncatedIDs[rwEdgeACLID]; !ok {
			t.Error("CloudFront ACL not marked uninspected after us-east-1 refused every call")
		}
	})
	t.Run("bucket encryption key, bucket-region S3 refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("us-east-1")
		assets := rwByID(t, rwS3Resources(t, w, clients), rwAssetsBucket)
		w.denyAccess("s3", "eu-west-1")
		rwRequireUnknown(t, rwChecker(t, "s3", "kms")(ctx, clients, assets, resource.ResourceCache{}))
	})
	t.Run("bucket posture, bucket-region S3 refused", func(t *testing.T) {
		w := newRegionWorld()
		clients := w.clients("us-east-1")
		assets := rwByID(t, rwS3Resources(t, w, clients), rwAssetsBucket)
		w.denyAccess("s3", "eu-west-1")
		res, _ := awsclient.EnrichS3Posture(ctx, clients, []resource.Resource{assets}, resource.ResourceCache{}) //nolint:errcheck // the refusal is expected; the row state is asserted
		if got := rwCodes(res.Findings[rwAssetsBucket]); len(got) != 0 {
			t.Errorf("findings = %v, want none: nothing was read", got)
		}
		if _, ok := res.TruncatedIDs[rwAssetsBucket]; !ok {
			t.Error("bucket not marked uninspected after its region refused every call")
		}
	})
}
