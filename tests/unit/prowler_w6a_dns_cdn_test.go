package unit

// prowler_w6a_dns_cdn_test.go — batch w6a rows 8-16: the Route 53 and
// CloudFront posture signals.
//
// All nine are Wave 2. Rows 9 and 10 are cache joins that make no AWS call of
// their own, so they are driven with a populated ResourceCache and an
// otherwise-idle client; the rest read the distribution config the detail
// path already fetches.
//
// The fakes embed the aggregate client interface and implement only the calls
// under test. A method the enricher must not reach panics on the nil
// embedded interface, which is a louder failure than a zero-value answer.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6aR53QueryLoggingOff = "r53.query-logging-off"
	w6aR53Dangling        = "r53.dangling-record"
	w6aCFOriginMissing    = "cf.origin-bucket-missing"
	w6aCFDeprecatedTLS    = "cf.deprecated-tls"
	w6aCFLoggingOff       = "cf.logging-off"
	w6aCFNoRootObject     = "cf.no-default-root-object"
	w6aCFNoOAC            = "cf.s3-origin-no-oac"
	w6aCFDefaultCert      = "cf.default-certificate"
	w6aCFNoGeoRestriction = "cf.no-geo-restriction"
)

// ---------------------------------------------------------------------------
// r53 — rows 8-9
// ---------------------------------------------------------------------------

// w6aR53Fake answers the three calls the zone enricher makes. Zones default to
// public with one query-logging config and no records, so each test switches
// off only the setting it is about.
type w6aR53Fake struct {
	awsclient.Route53API

	private    bool
	logConfigs int
	// logPages > 1 makes ListQueryLoggingConfigs paginate; logCalls counts
	// them so a walk that reads past its answer is visible.
	logPages int
	logCalls int
	// recordPages makes ListResourceRecordSets paginate forever, so the walk
	// runs into PerParentPageCap instead of ending.
	recordPages bool
	records     []r53types.ResourceRecordSet
	zoneErr     error
	recordsErr  error
	zoneCalls   int
	recordCalls int
}

func (f *w6aR53Fake) GetHostedZone(_ context.Context, in *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	f.zoneCalls++
	if f.zoneErr != nil {
		return nil, f.zoneErr
	}
	return &route53.GetHostedZoneOutput{
		HostedZone: &r53types.HostedZone{
			Id:                     in.Id,
			Name:                   aws.String("acme-corp.com."),
			Config:                 &r53types.HostedZoneConfig{PrivateZone: f.private},
			ResourceRecordSetCount: aws.Int64(int64(len(f.records))),
		},
		VPCs: []r53types.VPC{{VPCId: aws.String("vpc-0a1b2c3d4e5f60718"), VPCRegion: r53types.VPCRegionEuCentral1}},
	}, nil
}

func (f *w6aR53Fake) ListQueryLoggingConfigs(_ context.Context, _ *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	f.logCalls++
	out := &route53.ListQueryLoggingConfigsOutput{}
	if f.logCalls < f.logPages {
		out.NextToken = aws.String("next")
	}
	for i := 0; i < f.logConfigs; i++ {
		out.QueryLoggingConfigs = append(out.QueryLoggingConfigs, r53types.QueryLoggingConfig{
			Id:                        aws.String("qlc-0000000000000000"),
			HostedZoneId:              aws.String("Z0A1B2C3D4E5F6G7H8I9"),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/route53/acme-corp.com:*"),
		})
	}
	return out, nil
}

func (f *w6aR53Fake) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	f.recordCalls++
	if f.recordsErr != nil {
		return nil, f.recordsErr
	}
	out := &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.records}
	if f.recordPages {
		out.IsTruncated = true
		out.NextRecordName = aws.String("next.acme-corp.com.")
		out.NextRecordType = r53types.RRTypeA
	}
	return out, nil
}

// w6aARecord builds a realistic A record pointing at one address.
func w6aARecord(name, ip string) r53types.ResourceRecordSet {
	return r53types.ResourceRecordSet{
		Name:            aws.String(name),
		Type:            r53types.RRTypeA,
		TTL:             aws.Int64(300),
		ResourceRecords: []r53types.ResourceRecord{{Value: aws.String(ip)}},
	}
}

// w6aZoneRes builds the zone row shape the enricher consumes.
func w6aZoneRes(id, name string) resource.Resource {
	r := w2Res(id, nil)
	r.Name = name
	r.Fields["zone_id"] = id
	return r
}

func w6aEnrichR53(t *testing.T, fake *w6aR53Fake, cache resource.ResourceCache, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2Enricher(t, "r53")(context.Background(), &awsclient.ServiceClients{Route53: fake}, rs, cache)
	w2AssertEnricherShape(t, res)
	_ = err
	return res
}

// TestW6AR53QueryLoggingOff pins row 8. A public zone with no query-logging
// config leaves DNS lookups unrecorded, so an exfiltration channel over DNS
// has no trace to find.
func TestW6AR53QueryLoggingOff(t *testing.T) {
	off := w6aEnrichR53(t, &w6aR53Fake{logConfigs: 0}, nil, w6aZoneRes("Z0OFF00000000000000A", "no-query-logging.acme-corp.com."))
	w2AssertFinding(t, off.Findings["Z0OFF00000000000000A"], w6aR53QueryLoggingOff,
		"query logging off", domain.SevWarn, "wave2")
	// The phrase already states that logging is off; the row says why the
	// check applies to this zone at all (U11).
	w2AssertRow(t, w2Rows(t, off, "Z0OFF00000000000000A", w6aR53QueryLoggingOff), "Zone type", "public")

	on := w6aEnrichR53(t, &w6aR53Fake{logConfigs: 1}, nil, w6aZoneRes("Z0ON000000000000000A", "logged.acme-corp.com."))
	w2AssertNoCode(t, on.Findings["Z0ON000000000000000A"], w6aR53QueryLoggingOff)
}

// TestW6AR53QueryLoggingOff_PrivateZoneIsExempt pins the negative that matters
// most: private zones cannot have query logging, so flagging one would be an
// unfixable finding.
func TestW6AR53QueryLoggingOff_PrivateZoneIsExempt(t *testing.T) {
	res := w6aEnrichR53(t, &w6aR53Fake{private: true, logConfigs: 0}, nil,
		w6aZoneRes("Z0PRIV00000000000000", "internal.acme-corp."))
	w2AssertNoCode(t, res.Findings["Z0PRIV00000000000000"], w6aR53QueryLoggingOff)
}

// w6aAddressCache builds the eip/ec2 caches the dangling-record join reads.
// Row 6 of the parse spec: the verdict now reads the elastic IP's association
// state, so an eip fixture carries Fields["status"], and the eni cache is no
// longer consulted at all — the eni fetcher writes no public_ip, so requiring
// it voided the verdict without contributing an address.
func w6aAddressCache(truncated bool, unattachedEIPs, attachedEIPs, instanceIPs []string) resource.ResourceCache {
	mk := func(ips []string, status string) domain.ResourceCacheEntry {
		e := domain.ResourceCacheEntry{IsTruncated: truncated}
		for i, ip := range ips {
			fields := map[string]string{"public_ip": ip}
			if status != "" {
				fields["status"] = status
			}
			e.Resources = append(e.Resources, domain.Resource{
				ID:     "fixture-" + status + "-" + string(rune('a'+i)),
				Name:   ip,
				Fields: fields,
			})
		}
		return e
	}
	eip := mk(unattachedEIPs, "UNATTACHED")
	eip.Resources = append(eip.Resources, mk(attachedEIPs, "ATTACHED").Resources...)
	return resource.ResourceCache{
		"eip": eip,
		"ec2": mk(instanceIPs, ""),
	}
}

// TestW6AR53DanglingRecord pins row 9, as rewritten by row 6 of the parse
// spec. The finding is no longer "the address is absent from my inventory" —
// nothing read-only proves an absent address was ever this account's, and a
// name delegated to a CDN is absent for a perfectly good reason. What the
// inventory does prove is an elastic IP this account holds with nothing
// attached to it: every request for the name reaches an address that answers
// nobody. The old assertion is not to be restored.
func TestW6AR53DanglingRecord(t *testing.T) {
	fake := &w6aR53Fake{
		logConfigs: 1,
		records: []r53types.ResourceRecordSet{
			w6aARecord("dangling.acme-corp.com.", "203.0.113.11"),
			w6aARecord("live.acme-corp.com.", "203.0.113.10"),
			w6aARecord("cdn.acme-corp.com.", "198.51.100.50"),
		},
	}
	cache := w6aAddressCache(false, []string{"203.0.113.11"}, []string{"203.0.113.10"}, []string{"203.0.113.12"})

	res := w6aEnrichR53(t, fake, cache, w6aZoneRes("Z0DANGLING0000000000", "acme-corp.com."))

	w2AssertFinding(t, res.Findings["Z0DANGLING0000000000"], w6aR53Dangling,
		"record points at an unassociated elastic IP", domain.SevBroken, "wave2")
	rows := w2Rows(t, res, "Z0DANGLING0000000000", w6aR53Dangling)
	w2AssertRow(t, rows, "Record", "dangling.acme-corp.com.")
	w2AssertRow(t, rows, "Target", "203.0.113.11")
}

// TestW6AR53DanglingRecord_AnythingButAnIdleElasticIPIsClean pins the other
// side of row 6 of the parse spec. Three quite different situations all
// render clean, and only one of them is "the address is fine": an attached
// elastic IP and an instance address are reachable, while an address outside
// the account is simply not something this account can judge. The old
// version of this test asserted that an ENI address closes the finding; the
// eni fetcher writes no public_ip, so that leg never held an address and its
// assertion is not to be restored.
func TestW6AR53DanglingRecord_AnythingButAnIdleElasticIPIsClean(t *testing.T) {
	const addr = "203.0.113.201"
	for name, cache := range map[string]resource.ResourceCache{
		"an attached elastic IP":                w6aAddressCache(false, nil, []string{addr}, nil),
		"an instance address":                   w6aAddressCache(false, nil, nil, []string{addr}),
		"an address outside this account":       w6aAddressCache(false, nil, []string{"203.0.113.10"}, nil),
		"an idle elastic IP at another address": w6aAddressCache(false, []string{"203.0.113.99"}, nil, nil),
	} {
		t.Run(name, func(t *testing.T) {
			fake := &w6aR53Fake{logConfigs: 1, records: []r53types.ResourceRecordSet{
				w6aARecord("held.acme-corp.com.", addr),
			}}
			res := w6aEnrichR53(t, fake, cache, w6aZoneRes("Z0HELD00000000000000", "acme-corp.com."))
			w2AssertNoCode(t, res.Findings["Z0HELD00000000000000"], w6aR53Dangling)
		})
	}
}

// TestW6AR53DanglingRecord_IncompleteAddressCacheEmitsNothing pins the skip
// rule. A missing or truncated address cache means the address may well be
// held on a page nobody loaded, and calling that a takeover would send an
// operator chasing a record that is fine.
func TestW6AR53DanglingRecord_IncompleteAddressCacheEmitsNothing(t *testing.T) {
	full := w6aAddressCache(false, []string{"203.0.113.11"}, nil, nil)
	cases := map[string]resource.ResourceCache{
		"absent eip":    {"ec2": full["ec2"]},
		"absent ec2":    {"eip": full["eip"]},
		"all absent":    {},
		"truncated eip": w6aAddressCache(true, []string{"203.0.113.11"}, nil, nil),
	}
	for name, cache := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &w6aR53Fake{logConfigs: 1, records: []r53types.ResourceRecordSet{
				w6aARecord("dangling.acme-corp.com.", "203.0.113.11"),
			}}
			res := w6aEnrichR53(t, fake, cache, w6aZoneRes("Z0SKIP00000000000000", "acme-corp.com."))
			w2AssertNoCode(t, res.Findings["Z0SKIP00000000000000"], w6aR53Dangling)
		})
	}
}

// TestW6AR53DanglingRecord_OnlyPublicIPv4ARecords pins the three exclusions
// that would otherwise produce noise: private addresses are not claimable,
// AAAA and CNAME records are not covered by this check, and an alias record
// carries no address to compare.
func TestW6AR53DanglingRecord_OnlyPublicIPv4ARecords(t *testing.T) {
	alias := r53types.ResourceRecordSet{
		Name: aws.String("cdn.acme-corp.com."),
		Type: r53types.RRTypeA,
		AliasTarget: &r53types.AliasTarget{
			DNSName:      aws.String("d111111abcdef8.cloudfront.net."),
			HostedZoneId: aws.String("Z2FDTNDATAQYW2"),
		},
	}
	aaaa := w6aARecord("v6.acme-corp.com.", "2001:db8::1")
	aaaa.Type = r53types.RRTypeAaaa
	cname := w6aARecord("www.acme-corp.com.", "acme-corp.com.")
	cname.Type = r53types.RRTypeCname

	fake := &w6aR53Fake{logConfigs: 1, records: []r53types.ResourceRecordSet{
		w6aARecord("internal.acme-corp.com.", "10.0.4.17"),
		alias, aaaa, cname,
	}}
	res := w6aEnrichR53(t, fake, w6aAddressCache(false, []string{"203.0.113.10"}, nil, nil),
		w6aZoneRes("Z0QUIET00000000000000", "acme-corp.com."))

	w2AssertNoCode(t, res.Findings["Z0QUIET00000000000000"], w6aR53Dangling)
}

// TestW6AR53_RecordListingFailureTruncatesTheZone pins that a failed record
// listing marks the zone rather than reporting it clean. A zone whose records
// could not be read has an unknown posture, not a good one.
func TestW6AR53_RecordListingFailureTruncatesTheZone(t *testing.T) {
	fake := &w6aR53Fake{logConfigs: 1, recordsErr: errors.New("Throttling: Rate exceeded")}
	res := w6aEnrichR53(t, fake, w6aAddressCache(false, []string{"203.0.113.10"}, nil, nil),
		w6aZoneRes("Z0FAIL00000000000000", "acme-corp.com."))

	if _, marked := res.TruncatedIDs["Z0FAIL00000000000000"]; !marked {
		t.Error("a zone whose records could not be listed was not marked truncated")
	}
	w2AssertNoCode(t, res.Findings["Z0FAIL00000000000000"], w6aR53Dangling)
}

// TestW6AR53_CatalogDefs pins the catalog rows for both r53 codes.
func TestW6AR53_CatalogDefs(t *testing.T) {
	w2AssertFindingDef(t, "r53", w6aR53QueryLoggingOff, "query logging off", domain.SevWarn, "wave2")
	w2AssertFindingDef(t, "r53", w6aR53Dangling, "record points at an unassociated elastic IP", domain.SevBroken, "wave2")
}

// ---------------------------------------------------------------------------
// cf — rows 10-16
// ---------------------------------------------------------------------------

// w6aCFFake answers GetDistributionConfig with one config per distribution ID.
type w6aCFFake struct {
	awsclient.CloudFrontAPI

	configs map[string]*cftypes.DistributionConfig
	err     error
}

func (f *w6aCFFake) GetDistributionConfig(_ context.Context, in *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	cfg, ok := f.configs[aws.ToString(in.Id)]
	if !ok {
		return &cloudfront.GetDistributionConfigOutput{}, nil
	}
	return &cloudfront.GetDistributionConfigOutput{DistributionConfig: cfg}, nil
}

// w6aCFConfig returns a healthy distribution config: one S3 origin behind an
// origin access control, TLS 1.2 minimum on a custom certificate, access
// logging on, a default root object, and a geo restriction in place.
func w6aCFConfig(alias, originDomain string) *cftypes.DistributionConfig {
	return &cftypes.DistributionConfig{
		Enabled:           aws.Bool(true),
		Comment:           aws.String("acme storefront"),
		DefaultRootObject: aws.String("index.html"),
		Aliases: &cftypes.Aliases{
			Quantity: aws.Int32(1),
			Items:    []string{alias},
		},
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{{
				Id:                    aws.String("s3-origin"),
				DomainName:            aws.String(originDomain),
				OriginAccessControlId: aws.String("E1A2B3C4D5E6F7"),
				S3OriginConfig:        &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
			}},
		},
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("s3-origin"),
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
		},
		ViewerCertificate: &cftypes.ViewerCertificate{
			ACMCertificateArn:            aws.String("arn:aws:acm:us-east-1:123456789012:certificate/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
			CloudFrontDefaultCertificate: aws.Bool(false),
			MinimumProtocolVersion:       cftypes.MinimumProtocolVersionTLSv122021,
		},
		Logging: &cftypes.LoggingConfig{
			Enabled: aws.Bool(true),
			Bucket:  aws.String("acme-cdn-logs.s3.amazonaws.com"),
			Prefix:  aws.String("cf/"),
		},
		Restrictions: &cftypes.Restrictions{
			GeoRestriction: &cftypes.GeoRestriction{
				RestrictionType: cftypes.GeoRestrictionTypeWhitelist,
				Quantity:        aws.Int32(2),
				Items:           []string{"DE", "FR"},
			},
		},
	}
}

// w6aHeadBucketFake answers HeadBucket the way AWS answers for an account
// whose bucket list is exactly the s3 cache: a name outside it does not
// exist. Row 7 of the parse spec made this call the authority for "gone",
// because a cache listing only this account's buckets cannot speak for a
// bucket in another one.
type w6aHeadBucketFake struct {
	awsclient.S3API
	exists map[string]bool
}

func (f *w6aHeadBucketFake) HeadBucket(_ context.Context, in *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if f.exists[aws.ToString(in.Bucket)] {
		return &s3.HeadBucketOutput{}, nil
	}
	return nil, &s3types.NotFound{}
}

func w6aEnrichCF(t *testing.T, fake *w6aCFFake, cache resource.ResourceCache, ids ...string) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		rs = append(rs, w2Res(id, nil))
	}
	head := &w6aHeadBucketFake{exists: map[string]bool{}}
	for _, r := range cache["s3"].Resources {
		head.exists[r.ID] = true
	}
	res, err := w2Enricher(t, "cf")(context.Background(),
		&awsclient.ServiceClients{CloudFront: fake, S3: head}, rs, cache)
	w2AssertEnricherShape(t, res)
	_ = err
	return res
}

// w6aCFOne drives one distribution through the enricher with the given config.
func w6aCFOne(t *testing.T, id string, cfg *cftypes.DistributionConfig, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	return w6aEnrichCF(t, &w6aCFFake{configs: map[string]*cftypes.DistributionConfig{id: cfg}}, cache, id)
}

// w6aS3NameCache builds the s3 cache the origin-bucket join reads.
func w6aS3NameCache(truncated bool, names ...string) resource.ResourceCache {
	e := domain.ResourceCacheEntry{IsTruncated: truncated}
	for _, n := range names {
		e.Resources = append(e.Resources, domain.Resource{ID: n, Name: n})
	}
	return resource.ResourceCache{"s3": e}
}

// TestW6ACFOriginBucketMissing pins row 10. A distribution pointing at a
// bucket that no longer exists is a takeover risk: whoever creates that name
// next serves content from the account's own CDN domain.
func TestW6ACFOriginBucketMissing(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-deleted-origin.s3.us-east-1.amazonaws.com")
	res := w6aCFOne(t, "E6F7G8H9I0J1K2", cfg, w6aS3NameCache(false, "acme-live-origin"))

	w2AssertFinding(t, res.Findings["E6F7G8H9I0J1K2"], w6aCFOriginMissing,
		"S3 origin bucket does not exist", domain.SevBroken, "wave2")
	w2AssertRow(t, w2Rows(t, res, "E6F7G8H9I0J1K2", w6aCFOriginMissing),
		"Origin", "acme-deleted-origin.s3.us-east-1.amazonaws.com")

	present := w6aCFOne(t, "E6F7G8H9I0J1K2", cfg, w6aS3NameCache(false, "acme-deleted-origin"))
	w2AssertNoCode(t, present.Findings["E6F7G8H9I0J1K2"], w6aCFOriginMissing)
}

// TestW6ACFOriginBucketMissing_BothDomainShapes pins that the regional and
// the legacy global S3 domain forms are both recognised. Missing either shape
// hides the finding on half the real distributions.
func TestW6ACFOriginBucketMissing_BothDomainShapes(t *testing.T) {
	for _, domainName := range []string{
		"acme-gone.s3.us-east-1.amazonaws.com",
		"acme-gone.s3.amazonaws.com",
		"acme-gone.s3-website-us-east-1.amazonaws.com",
	} {
		t.Run(domainName, func(t *testing.T) {
			cfg := w6aCFConfig("shop.acme-corp.com", domainName)
			res := w6aCFOne(t, "E6F7G8H9I0J1K2", cfg, w6aS3NameCache(false, "acme-live-origin"))
			w2AssertFinding(t, res.Findings["E6F7G8H9I0J1K2"], w6aCFOriginMissing,
				"S3 origin bucket does not exist", domain.SevBroken, "wave2")
		})
	}
}

// TestW6ACFOriginBucketMissing_NonS3OriginIsNotChecked pins that a custom
// origin is out of scope: a9s cannot tell whether an arbitrary host exists.
func TestW6ACFOriginBucketMissing_NonS3OriginIsNotChecked(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "origin.acme-corp.com")
	cfg.Origins.Items[0].S3OriginConfig = nil
	cfg.Origins.Items[0].CustomOriginConfig = &cftypes.CustomOriginConfig{
		HTTPPort:             aws.Int32(80),
		HTTPSPort:            aws.Int32(443),
		OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpsOnly,
	}
	res := w6aCFOne(t, "E6F7G8H9I0J1K2", cfg, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, res.Findings["E6F7G8H9I0J1K2"], w6aCFOriginMissing)
}

// TestW6ACFOriginBucketMissing_IncompleteS3CacheEmitsNothing pins the skip
// rule: an absent or truncated bucket list is not evidence of a deleted
// bucket.
func TestW6ACFOriginBucketMissing_IncompleteS3CacheEmitsNothing(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-deleted-origin.s3.us-east-1.amazonaws.com")
	for name, cache := range map[string]resource.ResourceCache{
		"absent":    {},
		"truncated": w6aS3NameCache(true, "acme-live-origin"),
	} {
		t.Run(name, func(t *testing.T) {
			res := w6aCFOne(t, "E6F7G8H9I0J1K2", cfg, cache)
			w2AssertNoCode(t, res.Findings["E6F7G8H9I0J1K2"], w6aCFOriginMissing)
		})
	}
}

// TestW6ACFDeprecatedTLS pins row 11 across every protocol version AWS still
// accepts but no longer considers safe, and the two that are fine.
func TestW6ACFDeprecatedTLS(t *testing.T) {
	// Row 23: the value is the mapped word, never string(<SDK enum>). Two
	// enums share TLS 1.0, so those carry the policy year to stay distinct;
	// the acronym map already allows TLS.
	weak := map[cftypes.MinimumProtocolVersion]string{
		cftypes.MinimumProtocolVersionSSLv3:      "SSL 3.0",
		cftypes.MinimumProtocolVersionTLSv1:      "TLS 1.0",
		cftypes.MinimumProtocolVersionTLSv12016:  "TLS 1.0 (2016)",
		cftypes.MinimumProtocolVersionTLSv112016: "TLS 1.1 (2016)",
	}
	for version, want := range weak {
		t.Run(string(version), func(t *testing.T) {
			cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
			cfg.ViewerCertificate.MinimumProtocolVersion = version
			res := w6aCFOne(t, "E7G8H9I0J1K2L3", cfg, w6aS3NameCache(false, "acme-live-origin"))
			w2AssertFinding(t, res.Findings["E7G8H9I0J1K2L3"], w6aCFDeprecatedTLS,
				"minimum TLS below 1.2", domain.SevWarn, "wave2")
			w2AssertRow(t, w2Rows(t, res, "E7G8H9I0J1K2L3", w6aCFDeprecatedTLS),
				"Minimum TLS version", want)
		})
	}

	for _, version := range []cftypes.MinimumProtocolVersion{
		cftypes.MinimumProtocolVersionTLSv122018,
		cftypes.MinimumProtocolVersionTLSv122021,
	} {
		t.Run(string(version), func(t *testing.T) {
			cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
			cfg.ViewerCertificate.MinimumProtocolVersion = version
			res := w6aCFOne(t, "E7G8H9I0J1K2L3", cfg, w6aS3NameCache(false, "acme-live-origin"))
			w2AssertNoCode(t, res.Findings["E7G8H9I0J1K2L3"], w6aCFDeprecatedTLS)
		})
	}
}

// TestW6ACFLoggingOff pins row 12, including the nil case: CloudFront reports
// no logging block at all when logging was never configured, which is off
// rather than unknown.
func TestW6ACFLoggingOff(t *testing.T) {
	for name, mutate := range map[string]func(*cftypes.DistributionConfig){
		"disabled": func(c *cftypes.DistributionConfig) { c.Logging.Enabled = aws.Bool(false) },
		"nil flag": func(c *cftypes.DistributionConfig) { c.Logging.Enabled = nil },
		"no block": func(c *cftypes.DistributionConfig) { c.Logging = nil },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
			mutate(cfg)
			res := w6aCFOne(t, "E8H9I0J1K2L3M4", cfg, w6aS3NameCache(false, "acme-live-origin"))
			w2AssertFinding(t, res.Findings["E8H9I0J1K2L3M4"], w6aCFLoggingOff,
				"access logging off", domain.SevWarn, "wave2")
			// The phrase says logging is off; the row names the destination
			// that is missing (U11).
			w2AssertRow(t, w2Rows(t, res, "E8H9I0J1K2L3M4", w6aCFLoggingOff), "Log bucket", "none")
		})
	}

	on := w6aCFOne(t, "E8H9I0J1K2L3M4",
		w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com"),
		w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, on.Findings["E8H9I0J1K2L3M4"], w6aCFLoggingOff)
}

// TestW6ACFNoDefaultRootObject pins row 13. Without one, a request to the
// distribution root lists or exposes whatever the origin serves there.
func TestW6ACFNoDefaultRootObject(t *testing.T) {
	for name, value := range map[string]*string{"nil": nil, "empty": aws.String("")} {
		t.Run(name, func(t *testing.T) {
			cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
			cfg.DefaultRootObject = value
			res := w6aCFOne(t, "E9I0J1K2L3M4N5", cfg, w6aS3NameCache(false, "acme-live-origin"))
			w2AssertFinding(t, res.Findings["E9I0J1K2L3M4N5"], w6aCFNoRootObject,
				"no default root object", domain.SevWarn, "wave2")
			w2AssertRow(t, w2Rows(t, res, "E9I0J1K2L3M4N5", w6aCFNoRootObject), "Default root object", "none")
		})
	}

	set := w6aCFOne(t, "E9I0J1K2L3M4N5",
		w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com"),
		w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, set.Findings["E9I0J1K2L3M4N5"], w6aCFNoRootObject)
}

// TestW6ACFS3OriginWithoutOriginAccessControl pins row 14. Without an access
// control the bucket must be world-readable for the CDN to reach it, so the
// CDN stops being the only way in.
func TestW6ACFS3OriginWithoutOriginAccessControl(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	cfg.Origins.Items[0].OriginAccessControlId = aws.String("")
	cfg.Origins.Items[0].S3OriginConfig = &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")}

	res := w6aCFOne(t, "EA0J1K2L3M4N5O", cfg, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertFinding(t, res.Findings["EA0J1K2L3M4N5O"], w6aCFNoOAC,
		"S3 origin without origin access control", domain.SevWarn, "wave2")
	w2AssertRow(t, w2Rows(t, res, "EA0J1K2L3M4N5O", w6aCFNoOAC),
		"Origin", "acme-live-origin.s3.us-east-1.amazonaws.com")
}

// TestW6ACFS3OriginNoOAC_LegacyIdentityStillCounts pins that the older origin
// access identity closes the finding. It is deprecated, not absent, and
// flagging a distribution that already restricts its bucket is a false alarm.
func TestW6ACFS3OriginNoOAC_LegacyIdentityStillCounts(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	cfg.Origins.Items[0].OriginAccessControlId = aws.String("")
	cfg.Origins.Items[0].S3OriginConfig = &cftypes.S3OriginConfig{
		OriginAccessIdentity: aws.String("origin-access-identity/cloudfront/E1A2B3C4D5E6F7"),
	}
	res := w6aCFOne(t, "EA0J1K2L3M4N5O", cfg, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, res.Findings["EA0J1K2L3M4N5O"], w6aCFNoOAC)
}

// TestW6ACFDefaultCertificate pins row 15 and its exception: a distribution
// with no alias legitimately serves on its cloudfront.net name with the
// default certificate, and flagging it would be noise on every test stack.
func TestW6ACFDefaultCertificate(t *testing.T) {
	withAlias := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	withAlias.ViewerCertificate = &cftypes.ViewerCertificate{CloudFrontDefaultCertificate: aws.Bool(true)}
	res := w6aCFOne(t, "EB1K2L3M4N5O6P", withAlias, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertFinding(t, res.Findings["EB1K2L3M4N5O6P"], w6aCFDefaultCert,
		"uses the default CloudFront certificate", domain.SevWarn, "wave2")
	// The phrase names the certificate; the row names the custom domain that
	// makes the default certificate wrong here (U11).
	w2AssertRow(t, w2Rows(t, res, "EB1K2L3M4N5O6P", w6aCFDefaultCert), "Alias", "shop.acme-corp.com")

	noAlias := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	noAlias.ViewerCertificate = &cftypes.ViewerCertificate{CloudFrontDefaultCertificate: aws.Bool(true)}
	noAlias.Aliases = &cftypes.Aliases{Quantity: aws.Int32(0)}
	clean := w6aCFOne(t, "EB1K2L3M4N5O6P", noAlias, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, clean.Findings["EB1K2L3M4N5O6P"], w6aCFDefaultCert)
}

// TestW6ACFNoGeoRestriction pins row 16.
func TestW6ACFNoGeoRestriction(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	cfg.Restrictions = &cftypes.Restrictions{GeoRestriction: &cftypes.GeoRestriction{
		RestrictionType: cftypes.GeoRestrictionTypeNone,
		Quantity:        aws.Int32(0),
	}}
	res := w6aCFOne(t, "EC2L3M4N5O6P7Q", cfg, w6aS3NameCache(false, "acme-live-origin"))
	w2AssertFinding(t, res.Findings["EC2L3M4N5O6P7Q"], w6aCFNoGeoRestriction,
		"no geo restriction", domain.SevWarn, "wave2")
	w2AssertRow(t, w2Rows(t, res, "EC2L3M4N5O6P7Q", w6aCFNoGeoRestriction), "Countries", "none")

	restricted := w6aCFOne(t, "EC2L3M4N5O6P7Q",
		w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com"),
		w6aS3NameCache(false, "acme-live-origin"))
	w2AssertNoCode(t, restricted.Findings["EC2L3M4N5O6P7Q"], w6aCFNoGeoRestriction)
}

// TestW6ACF_ConditionsAreIndependent pins contract rule 4 across the six
// config-derived cf rows: a distribution that fails all of them carries six
// findings, each with its own supporting rows.
func TestW6ACF_ConditionsAreIndependent(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-deleted-origin.s3.us-east-1.amazonaws.com")
	cfg.DefaultRootObject = nil
	cfg.Logging = nil
	cfg.ViewerCertificate = &cftypes.ViewerCertificate{
		CloudFrontDefaultCertificate: aws.Bool(true),
		MinimumProtocolVersion:       cftypes.MinimumProtocolVersionTLSv1,
	}
	cfg.Origins.Items[0].OriginAccessControlId = aws.String("")
	cfg.Origins.Items[0].S3OriginConfig = &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")}
	cfg.Restrictions = &cftypes.Restrictions{GeoRestriction: &cftypes.GeoRestriction{
		RestrictionType: cftypes.GeoRestrictionTypeNone,
	}}

	res := w6aCFOne(t, "ED3M4N5O6P7Q8R", cfg, w6aS3NameCache(false, "acme-live-origin"))
	fs := res.Findings["ED3M4N5O6P7Q8R"]
	for _, code := range []string{
		w6aCFOriginMissing, w6aCFDeprecatedTLS, w6aCFLoggingOff,
		w6aCFNoRootObject, w6aCFNoOAC, w6aCFDefaultCert, w6aCFNoGeoRestriction,
	} {
		if _, ok := w2Find(fs, code); !ok {
			t.Errorf("missing %q; got %v", code, w2Codes(fs))
		}
	}
}

// TestW6ACF_ConfigFailureTruncatesTheDistribution pins that a distribution
// whose config could not be read is marked, not reported clean, and that the
// other distributions in the batch are still evaluated.
func TestW6ACF_ConfigFailureTruncatesTheDistribution(t *testing.T) {
	healthy := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	broken := w6aCFConfig("blog.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	broken.Logging = nil

	fake := &w6aCFFake{configs: map[string]*cftypes.DistributionConfig{
		"EE4N5O6P7Q8R9S": healthy,
		"EF5O6P7Q8R9S0T": broken,
	}}
	// The third ID has no config entry and no error: the fake returns an empty
	// output, which is how AWS answers a distribution deleted mid-pass.
	res := w6aEnrichCF(t, fake, w6aS3NameCache(false, "acme-live-origin"),
		"EE4N5O6P7Q8R9S", "EF5O6P7Q8R9S0T", "EG6P7Q8R9S0T1U")

	w2AssertFinding(t, res.Findings["EF5O6P7Q8R9S0T"], w6aCFLoggingOff,
		"access logging off", domain.SevWarn, "wave2")
	w2AssertNoCode(t, res.Findings["EE4N5O6P7Q8R9S"], w6aCFLoggingOff)
	w2AssertNoCode(t, res.Findings["EG6P7Q8R9S0T1U"], w6aCFLoggingOff)
}

// TestW6ACF_CapBoundary pins the enrichment cap: exactly EnrichmentCap
// distributions is not truncation, one more is.
func TestW6ACF_CapBoundary(t *testing.T) {
	cfg := w6aCFConfig("shop.acme-corp.com", "acme-live-origin.s3.us-east-1.amazonaws.com")
	cfg.Logging = nil

	ids := make([]string, 0, awsclient.EnrichmentCap+1)
	configs := make(map[string]*cftypes.DistributionConfig, awsclient.EnrichmentCap+1)
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		id := "EDIST" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		ids = append(ids, id)
		configs[id] = cfg
	}

	atCap := w6aEnrichCF(t, &w6aCFFake{configs: configs}, w6aS3NameCache(false, "acme-live-origin"), ids[:awsclient.EnrichmentCap]...)
	if atCap.Truncated {
		t.Error("exactly EnrichmentCap distributions reported Truncated")
	}
	overCap := w6aEnrichCF(t, &w6aCFFake{configs: configs}, w6aS3NameCache(false, "acme-live-origin"), ids...)
	if !overCap.Truncated {
		t.Error("EnrichmentCap+1 distributions did not report Truncated")
	}
}

// TestW6ACF_CatalogDefs pins the catalog rows for the seven cf codes.
func TestW6ACF_CatalogDefs(t *testing.T) {
	for _, c := range []struct {
		code, phrase string
		sev          domain.Severity
	}{
		{w6aCFOriginMissing, "S3 origin bucket does not exist", domain.SevBroken},
		{w6aCFDeprecatedTLS, "minimum TLS below 1.2", domain.SevWarn},
		{w6aCFLoggingOff, "access logging off", domain.SevWarn},
		{w6aCFNoRootObject, "no default root object", domain.SevWarn},
		{w6aCFNoOAC, "S3 origin without origin access control", domain.SevWarn},
		{w6aCFDefaultCert, "uses the default CloudFront certificate", domain.SevWarn},
		{w6aCFNoGeoRestriction, "no geo restriction", domain.SevWarn},
	} {
		w2AssertFindingDef(t, "cf", c.code, c.phrase, c.sev, "wave2")
	}
}

// TestW6AR53_RecordWalkStopsAtThePageCapAndSaysSo pins the paging the enricher
// gained when it stopped taking a skip-list entry: the walk stops at
// PerParentPageCap rather than running a zone of any size, and a zone it could
// not finish is marked truncated so the count reads as a lower bound.
//
// A dangling record seen in the prefix is still reported. More pages may hold
// more of them, which is what truncation says; none of that makes the one
// already found untrue, and staying silent would hide a live takeover because
// the zone is large.
func TestW6AR53_RecordWalkStopsAtThePageCapAndSaysSo(t *testing.T) {
	fake := &w6aR53Fake{
		logConfigs:  1,
		recordPages: true,
		records: []r53types.ResourceRecordSet{
			w6aARecord("dangling.acme-corp.com.", "203.0.113.11"),
		},
	}
	res := w6aEnrichR53(t, fake, w6aAddressCache(false, []string{"203.0.113.11"}, nil, nil),
		w6aZoneRes("Z0LONGZONE0000000000", "acme-corp.com."))

	if fake.recordCalls != awsclient.PerParentPageCap {
		t.Errorf("ListResourceRecordSets called %d times, want PerParentPageCap (%d)",
			fake.recordCalls, awsclient.PerParentPageCap)
	}
	if _, marked := res.TruncatedIDs["Z0LONGZONE0000000000"]; !marked {
		t.Error("a zone longer than the page cap was not marked truncated")
	}
	w2AssertFinding(t, res.Findings["Z0LONGZONE0000000000"], w6aR53Dangling,
		"record points at an unassociated elastic IP", domain.SevBroken, "wave2")
}

// TestW6AR53_QueryLoggingWalkStopsAtTheFirstConfig pins the other half: the
// row asks whether a zone has any config, so one on the first page answers it
// and every further page is a call nobody needed.
func TestW6AR53_QueryLoggingWalkStopsAtTheFirstConfig(t *testing.T) {
	found := &w6aR53Fake{logConfigs: 1, logPages: 5}
	res := w6aEnrichR53(t, found, nil, w6aZoneRes("Z0FIRSTHIT0000000000", "logged.acme-corp.com."))
	if found.logCalls != 1 {
		t.Errorf("ListQueryLoggingConfigs called %d times after the first hit, want 1", found.logCalls)
	}
	w2AssertNoCode(t, res.Findings["Z0FIRSTHIT0000000000"], w6aR53QueryLoggingOff)

	// A zone with no config anywhere has to be walked to the last page before
	// the finding is honest.
	empty := &w6aR53Fake{logConfigs: 0, logPages: 3}
	res = w6aEnrichR53(t, empty, nil, w6aZoneRes("Z0NOCONFIG0000000000", "unlogged.acme-corp.com."))
	if empty.logCalls != 3 {
		t.Errorf("ListQueryLoggingConfigs called %d times over 3 pages, want 3", empty.logCalls)
	}
	w2AssertFinding(t, res.Findings["Z0NOCONFIG0000000000"], w6aR53QueryLoggingOff,
		"query logging off", domain.SevWarn, "wave2")
}
