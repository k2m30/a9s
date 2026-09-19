// Behavioural coverage for the R53 related-resource checkers. They read one
// ListResourceRecordSets page per zone (r53ListRecordsFirstPage), which needs
// *ServiceClients with a non-nil Route53 field, and resolve targets through
// FetchRelatedTarget.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type fakeRoute53Full struct {
	// listRecordSetsOutput is returned for every ListResourceRecordSets call.
	listRecordSetsOutput *route53.ListResourceRecordSetsOutput
	listRecordSetsErr    error

	// getHostedZoneOutput is returned for GetHostedZone calls.
	getHostedZoneOutput *route53.GetHostedZoneOutput
	getHostedZoneErr    error
}

func (f *fakeRoute53Full) ListHostedZones(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{}, nil
}

func (f *fakeRoute53Full) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	if f.listRecordSetsErr != nil {
		return nil, f.listRecordSetsErr
	}
	if f.listRecordSetsOutput != nil {
		return f.listRecordSetsOutput, nil
	}
	return &route53.ListResourceRecordSetsOutput{}, nil
}

func (f *fakeRoute53Full) GetHostedZone(_ context.Context, _ *route53.GetHostedZoneInput, _ ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	if f.getHostedZoneErr != nil {
		return nil, f.getHostedZoneErr
	}
	if f.getHostedZoneOutput != nil {
		return f.getHostedZoneOutput, nil
	}
	return &route53.GetHostedZoneOutput{}, nil
}

func (f *fakeRoute53Full) ListQueryLoggingConfigs(_ context.Context, _ *route53.ListQueryLoggingConfigsInput, _ ...func(*route53.Options)) (*route53.ListQueryLoggingConfigsOutput, error) {
	return &route53.ListQueryLoggingConfigsOutput{}, nil
}

func (f *fakeRoute53Full) ListHostedZonesByVPC(_ context.Context, _ *route53.ListHostedZonesByVPCInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error) {
	return &route53.ListHostedZonesByVPCOutput{}, nil
}

var _ awsclient.Route53API = (*fakeRoute53Full)(nil)

// TestR53_CanonicalDNS_TrailingDot verifies that r53AliasDNSNames strips trailing dots
// and lowercases (exercised via checkR53APIGW's alias processing).
func TestR53_CanonicalDNS_TrailingDot(t *testing.T) {
	const apiID = "a1b2c3d4"
	aliasWithDot := "A1B2C3D4.execute-api.us-east-1.amazonaws.com."

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(aliasWithDot),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	apigwRes := resource.Resource{ID: apiID, Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apigwRes}},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z1EXAMPLE", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "apigw" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "apigw")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (trailing dot + uppercase must be normalized)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), apiID)
	}
}

// TestR53_CanonicalDNS_EmptyDNSName verifies that a nil AliasTarget is skipped.
func TestR53_CanonicalDNS_EmptyDNSName(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("example.com."),
					Type: r53types.RRTypeA,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("1.2.3.4")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z1EMPTY", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no alias records)", result.Count())
	}
}

// TestRelated_R53_APIGW_Match verifies that an alias record pointing at
// "<api-id>.execute-api.<region>.amazonaws.com" yields the API Gateway ID from cache.
func TestRelated_R53_APIGW_Match(t *testing.T) {
	const apiID = "xyz99887766"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("api.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(apiID + ".execute-api.us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	apigwRes := resource.Resource{
		ID:     apiID,
		Name:   "my-api",
		Fields: map[string]string{"name": "my-api", "protocol": "HTTP"},
	}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apigwRes}},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z2APIGWTEST", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "apigw" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "apigw")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), apiID)
	}
}

// TestRelated_R53_APIGW_NoMatch verifies that alias records pointing at non-APIGW
// endpoints yield Count:0.
func TestRelated_R53_APIGW_NoMatch(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("myalb-123456.us-east-1.elb.amazonaws.com"),
						EvaluateTargetHealth: true,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "aaabbbccc", Fields: map[string]string{}},
			},
		},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z3NOAPI", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alias does not point at execute-api domain)", result.Count())
	}
}

// TestRelated_R53_APIGW_NilClients verifies that nil clients → State: RelatedUnknown.
func TestRelated_R53_APIGW_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z4NILCLIENTS", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count())
	}
}

func TestRelated_R53_APIGW_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count())
	}
}

// An apigw cache entry that is present with a nil list is a resolved zero.
func TestRelated_R53_APIGW_CacheNilList(t *testing.T) {
	const apiID = "directid12345"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("api.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(apiID + ".execute-api.eu-west-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	// An alias DNS name is not an apigw ID, so it never becomes a row. A present
	// entry is a complete answer even when empty (FetchRelatedTarget's cache-hit
	// path); Unknown is reserved for a cache miss with no fetcher
	// (r53_related_nil_cache_test.go).
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: nil},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z5NILLIST", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if got := result.EffectiveState(); got != domain.RelatedResolved {
		t.Errorf("state = %v, want RelatedResolved: a present cache entry is a complete answer", got)
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 — the account holds no APIs, so the zone's aliases resolve to none", result.Count())
	}
	if len(result.ResourceIDs()) != 0 {
		t.Errorf("ResourceIDs = %v, want none — %q is parsed out of an alias hostname, not an apigw ID",
			result.ResourceIDs(), apiID)
	}
}

// An alias to the dash-style S3 website endpoint resolves to the bucket named
// by the RECORD. An S3 website alias target is the bare regional endpoint and
// never carries the bucket, so the join key is the record name.
func TestRelated_R53_S3_Match(t *testing.T) {
	const bucketName = "my-website-bucket"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String(bucketName + "."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("s3-website-us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	s3Res := resource.Resource{
		ID:     bucketName,
		Name:   bucketName,
		Fields: map[string]string{"name": bucketName, "region": "us-east-1"},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "Z6S3TEST", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "s3" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "s3")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), bucketName)
	}
}

// The dot-style "s3-website.<region>" endpoint also resolves to the bucket
// named by the record.
func TestRelated_R53_S3_NewStyleEndpoint(t *testing.T) {
	const bucketName = "another-bucket-2024"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String(bucketName + "."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("s3-website.us-west-2.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	s3Res := resource.Resource{
		ID:     bucketName,
		Fields: map[string]string{"name": bucketName},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "Z7S3NEW", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (new-style s3-website endpoint)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), bucketName)
	}
}

// TestRelated_R53_S3_NoMatch verifies that non-S3-website aliases yield Count:0.
func TestRelated_R53_S3_NoMatch(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("api.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("abc123.execute-api.us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "some-bucket", Fields: map[string]string{}}},
		},
	}

	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "Z8NOS3", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alias not pointing at s3-website endpoint)", result.Count())
	}
}

// TestRelated_R53_S3_NilClients verifies that nil clients → State: RelatedUnknown.
func TestRelated_R53_S3_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "Z9NILS3", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil clients → errClientMissing)", result.State())
	}
}

func TestRelated_R53_S3_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count())
	}
}

// checkR53Logs returns State: RelatedUnknown for a non-empty zone ID.
func TestRelated_R53_Logs_Unknown(t *testing.T) {
	checker := r53CheckerByTarget(t, "logs")
	source := resource.Resource{ID: "ZALOGS123", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.TargetType() != "logs" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "logs")
	}
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (ListQueryLoggingConfigs not yet wired)", result.Count())
	}
}

func TestRelated_R53_Logs_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "logs")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count())
	}
}

// TestRelated_R53_VPC_PrivateZone_Match verifies that checkR53VPC calls GetHostedZone
// and returns the VPCs associated with a private hosted zone.
func TestRelated_R53_VPC_PrivateZone_Match(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		getHostedZoneOutput: &route53.GetHostedZoneOutput{
			VPCs: []r53types.VPC{
				{VPCId: aws.String("vpc-0a1b2c3d4e5f00001"), VPCRegion: r53types.VPCRegionUsEast1},
				{VPCId: aws.String("vpc-0a1b2c3d4e5f00002"), VPCRegion: r53types.VPCRegionUsEast1},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	source := resource.Resource{
		ID:   "/hostedzone/ZPRIVATE001",
		Name: "internal.example.com.",
		Fields: map[string]string{
			"private_zone": "true",
		},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), clients, source, resource.ResourceCache{})

	if result.TargetType() != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "vpc")
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	// ResourceIDs are sorted by relatedResult.
	wantIDs := []string{"vpc-0a1b2c3d4e5f00001", "vpc-0a1b2c3d4e5f00002"}
	for i, id := range wantIDs {
		if i >= len(result.ResourceIDs()) || result.ResourceIDs()[i] != id {
			t.Errorf("ResourceIDs[%d] = %q, want %q", i, func() string {
				if i < len(result.ResourceIDs()) {
					return result.ResourceIDs()[i]
				}
				return "<missing>"
			}(), id)
		}
	}
}

// TestRelated_R53_VPC_PublicZone_ReturnsZero verifies that a public hosted zone
// (private_zone != "true") immediately returns Count:0 without an API call.
func TestRelated_R53_VPC_PublicZone_ReturnsZero(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		// Returned only if the checker calls GetHostedZone, which fails the Count:0
		// assertion.
		getHostedZoneOutput: &route53.GetHostedZoneOutput{
			VPCs: []r53types.VPC{
				{VPCId: aws.String("vpc-unexpected")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	source := resource.Resource{
		ID:   "ZPUBLIC001",
		Name: "public.example.com.",
		Fields: map[string]string{
			"private_zone": "false",
		},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (public zone has no VPC associations)", result.Count())
	}
}

// TestRelated_R53_VPC_PrivateZone_NilClients verifies State: RelatedUnknown when clients are nil.
func TestRelated_R53_VPC_PrivateZone_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:     "ZPRIVATE002",
		Fields: map[string]string{"private_zone": "true"},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → Route53 not initialized)", result.Count())
	}
}

// TestRelated_R53_VPC_EmptyID verifies Count:0 when zone ID is empty.
func TestRelated_R53_VPC_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:     "",
		Fields: map[string]string{"private_zone": "true"},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count())
	}
}

// TestRelated_R53_VPC_DuplicateVPCIDs verifies that duplicate VPC IDs in GetHostedZone
// are deduplicated in the result.
func TestRelated_R53_VPC_DuplicateVPCIDs(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		getHostedZoneOutput: &route53.GetHostedZoneOutput{
			VPCs: []r53types.VPC{
				{VPCId: aws.String("vpc-0dedup001"), VPCRegion: r53types.VPCRegionUsEast1},
				{VPCId: aws.String("vpc-0dedup001"), VPCRegion: r53types.VPCRegionUsEast1}, // duplicate
				{VPCId: aws.String("vpc-0dedup002"), VPCRegion: r53types.VPCRegionUsEast1},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	source := resource.Resource{
		ID:     "ZPRIVATE003",
		Fields: map[string]string{"private_zone": "true"},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (duplicates must be deduplicated)", result.Count())
	}
}

// TestRelated_R53_APIGW_MultipleRecords_OnlyAPIDNSMatches verifies that a zone
// with mixed alias records (ELB + APIGW) only returns APIGW entries.
func TestRelated_R53_APIGW_MultipleRecords_OnlyAPIDNSMatches(t *testing.T) {
	const apiID = "multimatch9911"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(apiID + ".execute-api.us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
				{
					Name: aws.String("lb.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("myalb-123.us-east-1.elb.amazonaws.com"),
						EvaluateTargetHealth: true,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	apigwRes := resource.Resource{ID: apiID, Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apigwRes}},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "ZMULTI", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (only APIGW alias should match)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), apiID)
	}
}

// TestRelated_R53_ELB_Match verifies that alias records pointing at an ELB DNS
// name (*.elb.amazonaws.com) resolve to the matching ELB ID from cache.
func TestRelated_R53_ELB_Match(t *testing.T) {
	const elbDNS = "myalb-123456789.us-east-1.elb.amazonaws.com"
	const elbID = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/myalb/abc123"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(elbDNS),
						EvaluateTargetHealth: true,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	elbRes := resource.Resource{
		ID:   elbID,
		Name: "myalb",
		Fields: map[string]string{
			"dns_name": elbDNS,
		},
	}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}

	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB001", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "elb")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != elbID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), elbID)
	}
}

// TestRelated_R53_ELB_DualstackPrefix verifies that alias DNS names prefixed
// with "dualstack." are matched against the unprefixed ELB dns_name in cache.
func TestRelated_R53_ELB_DualstackPrefix(t *testing.T) {
	const elbDNS = "myalb-123456789.us-east-1.elb.amazonaws.com"
	const elbID = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/myalb/abc123"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeAaaa,
					AliasTarget: &r53types.AliasTarget{
						// Route53 often emits "dualstack.<elb-dns>" for IPv6 records.
						DNSName:              aws.String("dualstack." + elbDNS),
						EvaluateTargetHealth: true,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	elbRes := resource.Resource{
		ID:     elbID,
		Fields: map[string]string{"dns_name": elbDNS},
	}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}

	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB002", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (dualstack. prefix must match unprefixed dns_name)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != elbID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), elbID)
	}
}

// TestRelated_R53_ELB_NoMatch verifies that alias records pointing at non-ELB
// endpoints yield Count:0.
func TestRelated_R53_ELB_NoMatch(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("api.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("abc123.execute-api.us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "some-elb", Fields: map[string]string{"dns_name": "some-elb.us-east-1.elb.amazonaws.com"}}},
		},
	}

	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB003", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alias does not point at .elb.amazonaws.com)", result.Count())
	}
}

// A truncated record scan (ListResourceRecordSets IsTruncated=true) with no ELB
// alias found reports "(0+)", not "(0)": a confident "elb (0)" from a partial
// scan reads as "nothing points at this ELB".
func TestRelated_R53_ELB_NoMatch_TruncatedScan(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("api.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("abc123.execute-api.us-east-1.amazonaws.com"),
						EvaluateTargetHealth: false,
					},
				},
			},
			IsTruncated: true,
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "some-elb", Fields: map[string]string{"dns_name": "some-elb.us-east-1.elb.amazonaws.com"}}},
		},
	}

	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB004", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alias does not point at .elb.amazonaws.com)", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true: the record scan was truncated, so \"no match\" is not proven — must render (0+), not (0)")
	}
}

// A truncated record scan reports the matches found so far as truncated, not
// as the exhaustive set.
func TestRelated_R53_ELB_Match_TruncatedScan(t *testing.T) {
	const elbDNS = "myalb-123456789.us-east-1.elb.amazonaws.com"
	const elbID = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/myalb/abc123"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(elbDNS),
						EvaluateTargetHealth: true,
					},
				},
			},
			IsTruncated: true,
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	elbRes := resource.Resource{ID: elbID, Name: "myalb", Fields: map[string]string{"dns_name": elbDNS}}
	cache := resource.ResourceCache{"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}}}

	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB005", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true: the record scan was truncated, so this match set is not proven exhaustive")
	}
}

// TestRelated_R53_ELB_NilClients verifies that nil clients → State: RelatedUnknown.
func TestRelated_R53_ELB_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "elb")
	source := resource.Resource{ID: "ZELB004", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count())
	}
}

// TestRelated_R53_CF_Match verifies that alias records pointing at a CloudFront
// distribution domain (*.cloudfront.net) resolve to the CF ID from cache.
func TestRelated_R53_CF_Match(t *testing.T) {
	const cfDomain = "d1234abcdef.cloudfront.net"
	const cfID = "E1ABCDEF123456"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("www.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(cfDomain),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	cfRes := resource.Resource{
		ID:   cfID,
		Name: cfID,
		Fields: map[string]string{
			"domain_name": cfDomain,
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}

	checker := r53CheckerByTarget(t, "cf")
	source := resource.Resource{ID: "ZCF001", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "cf")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != cfID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), cfID)
	}
}

// TestRelated_R53_CF_NoMatch verifies that alias records pointing at non-CF
// endpoints yield Count:0.
func TestRelated_R53_CF_NoMatch(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("app.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("myalb-123.us-east-1.elb.amazonaws.com"),
						EvaluateTargetHealth: true,
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "EDIST001", Fields: map[string]string{"domain_name": "d999xyz.cloudfront.net"}}},
		},
	}

	checker := r53CheckerByTarget(t, "cf")
	source := resource.Resource{ID: "ZCF002", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (alias does not point at .cloudfront.net)", result.Count())
	}
}

// TestRelated_R53_CF_NilClients verifies that nil clients → State: RelatedUnknown.
func TestRelated_R53_CF_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "cf")
	source := resource.Resource{ID: "ZCF003", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count())
	}
}

// r53ACMCache holds one loaded certificate per domain, keyed on its ARN as
// the acm list keys its rows.
func r53ACMCache(domains ...string) (resource.ResourceCache, []string) {
	var rows []resource.Resource
	var arns []string
	for i, d := range domains {
		arn := "arn:aws:acm:us-east-1:123456789012:certificate/0000000" + string(rune('1'+i))
		arns = append(arns, arn)
		rows = append(rows, resource.Resource{ID: arn, RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(arn),
			DomainName:     aws.String(d),
		}})
	}
	return resource.ResourceCache{"acm": resource.ResourceCacheEntry{Resources: rows}}, arns
}

// TestRelated_R53_ACM_Match verifies that a CNAME record starting with "_" whose
// value ends with ".acm-validations.aws" counts the certificate for the domain
// it validates. The record name is no acm row, so the ID is the certificate
// ARN.
func TestRelated_R53_ACM_Match(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					// ACM DNS validation record: _<token>.<domain>. CNAME <token>.acm-validations.aws.
					Name: aws.String("_acmchallenge.example.com."),
					Type: r53types.RRTypeCname,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("_abc123def456.acm-validations.aws.")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	cache, arns := r53ACMCache("example.com", "other.example.org")

	checker := r53CheckerByTarget(t, "acm")
	source := resource.Resource{ID: "ZACM001", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType() != "acm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "acm")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != arns[0] {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), arns[0])
	}
}

// TestRelated_R53_ACM_MultipleValidationRecords verifies that two CNAME validation
// records for two certificates in one zone both produce IDs: the
// certificates' ARNs.
func TestRelated_R53_ACM_MultipleValidationRecords(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("_cert1.example.com."),
					Type: r53types.RRTypeCname,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("_token1.acm-validations.aws.")},
					},
				},
				{
					Name: aws.String("_cert2.www.example.com."),
					Type: r53types.RRTypeCname,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("_token2.acm-validations.aws.")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	cache, arns := r53ACMCache("example.com", "www.example.com")

	checker := r53CheckerByTarget(t, "acm")
	source := resource.Resource{ID: "ZACM002", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (two distinct validation records)", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, arn := range arns {
		if !seen[arn] {
			t.Errorf("ResourceIDs missing %s; got %v", arn, result.ResourceIDs())
		}
	}
}

// TestRelated_R53_ACM_NonACMCNAMEIgnored verifies that CNAME records not matching
// the ACM validation pattern (_prefix + .acm-validations.aws suffix) are ignored.
func TestRelated_R53_ACM_NonACMCNAMEIgnored(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					// CNAME without _ prefix — not ACM validation.
					Name: aws.String("mail.example.com."),
					Type: r53types.RRTypeCname,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("mailserver.example.com.")},
					},
				},
				{
					// CNAME with _ prefix but wrong suffix — not ACM.
					Name: aws.String("_dmarc.example.com."),
					Type: r53types.RRTypeCname,
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("v=DMARC1; p=none")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fakeR53}

	checker := r53CheckerByTarget(t, "acm")
	source := resource.Resource{ID: "ZACM003", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (non-ACM CNAMEs must not be counted)", result.Count())
	}
}

// TestRelated_R53_ACM_NilClients verifies that nil clients → State: RelatedUnknown.
func TestRelated_R53_ACM_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "acm")
	source := resource.Resource{ID: "ZACM004", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count())
	}
}
