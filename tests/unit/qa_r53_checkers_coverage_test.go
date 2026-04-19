// qa_r53_checkers_coverage_test.go — Behavioral coverage tests for R53 related-resource checkers.
//
// Tests cover: r53AliasDNSNames (helper), canonicalDNS (helper),
// checkR53APIGW, checkR53S3, checkR53Logs, checkR53VPC.
//
// These checkers use r53ListRecordsFirstPage (single ListResourceRecordSets call per zone)
// and FetchRelatedTarget for cache resolution. r53ListRecordsFirstPage requires
// *ServiceClients with a non-nil Route53 field implementing Route53API.
//
// Tests should PASS against current main — they cover existing, correct code.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeRoute53Full implements awsclient.Route53API for testing R53 checkers.
// Route53API = Route53ListHostedZonesAPI + Route53ListResourceRecordSetsAPI + Route53GetHostedZoneAPI.
// ---------------------------------------------------------------------------

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

// Compile-time check: fakeRoute53Full satisfies Route53API.
var _ awsclient.Route53API = (*fakeRoute53Full)(nil)

// ---------------------------------------------------------------------------
// canonicalDNS helper tests (pure function, no AWS calls needed).
// These call the checker through the public API: via r53AliasDNSNames behavior,
// exercised indirectly by the checker tests below.
// Direct unit tests use a synthetic record set.
// ---------------------------------------------------------------------------

// r53CheckerByTargetFull is a copy-compatible accessor for this file's tests
// (r53CheckerByTarget is already declared in aws_r53_related_test.go in the same package).

// TestR53_CanonicalDNS_TrailingDot verifies that r53AliasDNSNames strips trailing dots
// and lowercases (exercised via checkR53APIGW's alias processing).
func TestR53_CanonicalDNS_TrailingDot(t *testing.T) {
	// We exercise canonicalDNS indirectly through the checker:
	// a record with AliasTarget.DNSName = "A1B2C3D4.execute-api.us-east-1.amazonaws.com."
	// (trailing dot, uppercase) should still be matched against the api cache entry "a1b2c3d4".
	const apiID = "a1b2c3d4"
	aliasWithDot := "A1B2C3D4.execute-api.us-east-1.amazonaws.com." // uppercase + trailing dot

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

	// APIGW cache contains resource with ID matching the API ID extracted from the alias.
	apigwRes := resource.Resource{ID: apiID, Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apigwRes}},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z1EXAMPLE", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.TargetType != "apigw" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "apigw")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (trailing dot + uppercase must be normalized)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, apiID)
	}
}

// TestR53_CanonicalDNS_EmptyDNSName verifies that a nil AliasTarget is skipped.
func TestR53_CanonicalDNS_EmptyDNSName(t *testing.T) {
	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					// No AliasTarget — should not contribute to alias names.
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

	// No alias DNS names → Count:0, not -1.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no alias records)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkR53APIGW tests
// ---------------------------------------------------------------------------

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

	if result.TargetType != "apigw" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "apigw")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, apiID)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (alias does not point at execute-api domain)", result.Count)
	}
}

// TestRelated_R53_APIGW_NilClients verifies that nil clients → Count:-1.
func TestRelated_R53_APIGW_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z4NILCLIENTS", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count)
	}
}

func TestRelated_R53_APIGW_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count)
	}
}

// TestRelated_R53_APIGW_CacheNilList verifies that when the apigw cache entry has a
// nil resource list (loaded but empty), the extracted API IDs from the alias hostname
// are used as resource IDs directly.
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

	// Cache entry present but empty resource list — checker uses DNS-extracted IDs as fallback.
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: nil},
	}

	checker := r53CheckerByTarget(t, "apigw")
	source := resource.Resource{ID: "Z5NILLIST", Fields: map[string]string{}}
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (nil cache list → IDs from DNS hostname)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, apiID)
	}
}

// ---------------------------------------------------------------------------
// checkR53S3 tests
// ---------------------------------------------------------------------------

// TestRelated_R53_S3_Match verifies that alias records for an S3 website endpoint
// (<bucket>.s3-website-<region>.amazonaws.com) resolve to the bucket name in cache.
func TestRelated_R53_S3_Match(t *testing.T) {
	const bucketName = "my-website-bucket"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("www.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(bucketName + ".s3-website-us-east-1.amazonaws.com"),
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

	if result.TargetType != "s3" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "s3")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bucketName)
	}
}

// TestRelated_R53_S3_NewStyleEndpoint verifies alias records using the newer
// "s3-website.<region>" (no hyphen between s3-website and region) style.
func TestRelated_R53_S3_NewStyleEndpoint(t *testing.T) {
	const bucketName = "another-bucket-2024"

	fakeR53 := &fakeRoute53Full{
		listRecordSetsOutput: &route53.ListResourceRecordSetsOutput{
			ResourceRecordSets: []r53types.ResourceRecordSet{
				{
					Name: aws.String("site.example.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						// Newer endpoint style: s3-website.<region>.amazonaws.com
						DNSName:              aws.String(bucketName + ".s3-website.us-west-2.amazonaws.com"),
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (new-style s3-website endpoint)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bucketName)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (alias not pointing at s3-website endpoint)", result.Count)
	}
}

// TestRelated_R53_S3_NilClients verifies that nil clients → Count:-1.
func TestRelated_R53_S3_NilClients(t *testing.T) {
	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "Z9NILS3", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → errClientMissing)", result.Count)
	}
}

func TestRelated_R53_S3_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "s3")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkR53Logs tests
// ---------------------------------------------------------------------------

// TestRelated_R53_Logs_Unknown verifies that checkR53Logs always returns Count:-1
// for a non-empty zone ID (ListQueryLoggingConfigs not yet wired).
func TestRelated_R53_Logs_Unknown(t *testing.T) {
	checker := r53CheckerByTarget(t, "logs")
	source := resource.Resource{ID: "ZALOGS123", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.TargetType != "logs" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "logs")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ListQueryLoggingConfigs not yet wired)", result.Count)
	}
}

func TestRelated_R53_Logs_EmptyID(t *testing.T) {
	checker := r53CheckerByTarget(t, "logs")
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkR53VPC tests
// ---------------------------------------------------------------------------

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

	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	// ResourceIDs are sorted by relatedResult.
	wantIDs := []string{"vpc-0a1b2c3d4e5f00001", "vpc-0a1b2c3d4e5f00002"}
	for i, id := range wantIDs {
		if i >= len(result.ResourceIDs) || result.ResourceIDs[i] != id {
			t.Errorf("ResourceIDs[%d] = %q, want %q", i, func() string {
				if i < len(result.ResourceIDs) {
					return result.ResourceIDs[i]
				}
				return "<missing>"
			}(), id)
		}
	}
}

// TestRelated_R53_VPC_PublicZone_ReturnsZero verifies that a public hosted zone
// (private_zone != "true") immediately returns Count:0 without an API call.
func TestRelated_R53_VPC_PublicZone_ReturnsZero(t *testing.T) {
	// If the checker ignores the fake (as expected for public zones), Count:0.
	fakeR53 := &fakeRoute53Full{
		// If called unexpectedly, return some VPCs to ensure the test detects the bug.
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (public zone has no VPC associations)", result.Count)
	}
}

// TestRelated_R53_VPC_PrivateZone_NilClients verifies Count:-1 when clients are nil.
func TestRelated_R53_VPC_PrivateZone_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:     "ZPRIVATE002",
		Fields: map[string]string{"private_zone": "true"},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → Route53 not initialized)", result.Count)
	}
}

// TestRelated_R53_VPC_EmptyID verifies Count:0 when zone ID is empty.
func TestRelated_R53_VPC_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:     "",
		Fields: map[string]string{"private_zone": "true"},
	}
	result := r53CheckerByTarget(t, "vpc")(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone ID)", result.Count)
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

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (duplicates must be deduplicated)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// Multi-record alias tests — verify that only matching alias types are returned.
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only APIGW alias should match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, apiID)
	}
}
