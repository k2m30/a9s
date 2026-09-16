package unit_test

// s3_0916_dev_edges_test.go — the edges of task s3-0916's five rows that no
// row test pins: the negated form of each rule, the empty input, the malformed
// element, and the partition the examples do not use.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Row 3 — which part of the record names the bucket
// ---------------------------------------------------------------------------

// TestS3_0916_Dev_Row3_BucketPrefixedTargetNamesNoBucket pins the ruling that a
// bucket segment inside the alias target is not a bucket. S3 routes a website
// request by Host header, so AWS requires the record's name to equal the
// bucket; a target carrying a bucket segment is not a shape Route 53 returns,
// and matching it puts an unrelated bucket in the zone's related panel.
func TestS3_0916_Dev_Row3_BucketPrefixedTargetNamesNoBucket(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{
			row3AliasRecord("downloads.example.com.", row3SiteBucket+".s3-website-us-east-1.amazonaws.com."),
		},
		row3S3Cache(false, row3SiteBucket),
	)
	if got.Count() != 0 {
		t.Fatalf("Count = %d, want 0 — the bucket is the record's name, never a segment of the target", got.Count())
	}
}

// TestS3_0916_Dev_Row3_RecordNameIsLowercased pins the join key's case: Route
// 53 answers with the name as stored, and a bucket id is lowercase.
func TestS3_0916_Dev_Row3_RecordNameIsLowercased(t *testing.T) {
	got := row3Run(t,
		[]r53types.ResourceRecordSet{row3AliasRecord("ACME-Site.Example.COM.", "s3-website-us-east-1.amazonaws.com.")},
		row3S3Cache(false, row3SiteBucket),
	)
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != row3SiteBucket {
		t.Fatalf("ResourceIDs = %v, want [%s]", ids, row3SiteBucket)
	}
}

// TestS3_0916_Dev_Row3_FetcherAndPivotAgree pins the two sides of the join
// reading one derivation: the zone field the reverse pivot joins on and the
// forward pivot's own answer name the same bucket.
func TestS3_0916_Dev_Row3_FetcherAndPivotAgree(t *testing.T) {
	sets := []r53types.ResourceRecordSet{
		row3AliasRecord(row3SiteBucket+".", "s3-website-us-east-1.amazonaws.com."),
		row3AliasRecord("cdn.example.com.", "d111111abcdef8.cloudfront.net."),
	}
	fetched, err := awsclient.FetchHostedZonesPage(context.Background(), &row3ZoneFake{sets: sets}, "")
	if err != nil {
		t.Fatalf("fetch error = %v, want nil", err)
	}
	if len(fetched.Resources) != 1 {
		t.Fatalf("Resources = %d, want 1 zone", len(fetched.Resources))
	}
	if got := fetched.Resources[0].Fields["s3website_alias_names"]; got != row3SiteBucket {
		t.Errorf("Fields[\"s3website_alias_names\"] = %q, want %q", got, row3SiteBucket)
	}

	pivot := row3Run(t, sets, row3S3Cache(false, row3SiteBucket))
	if ids := pivot.ResourceIDs(); len(ids) != 1 || ids[0] != row3SiteBucket {
		t.Errorf("r53→s3 ResourceIDs = %v, want [%s] — the two sides read one derivation", ids, row3SiteBucket)
	}
}

// row3ZoneFake answers the two calls the zone fetcher makes.
type row3ZoneFake struct {
	awsclient.Route53API
	sets []r53types.ResourceRecordSet
}

func (f *row3ZoneFake) ListHostedZones(
	_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options),
) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{
		HostedZones: []r53types.HostedZone{{
			Id:                     aws.String(row3ZoneID),
			Name:                   aws.String("example.com."),
			ResourceRecordSetCount: aws.Int64(int64(len(f.sets) + 2)),
			Config:                 &r53types.HostedZoneConfig{PrivateZone: false},
		}},
	}, nil
}

func (f *row3ZoneFake) ListResourceRecordSets(
	_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.sets}, nil
}

// ---------------------------------------------------------------------------
// Row 4 — hostnames the stories do not cover
// ---------------------------------------------------------------------------

// TestS3_0916_Dev_Row4_ChinaPartitionOriginIsAnS3Endpoint pins the other
// partition: a China endpoint is an S3 endpoint exactly as its commercial twin.
func TestS3_0916_Dev_Row4_ChinaPartitionOriginIsAnS3Endpoint(t *testing.T) {
	dist := row4Distribution("acme-cn-assets.s3.cn-north-1.amazonaws.com.cn")
	got := checkerByTarget(t, "cf", "s3")(context.Background(), nil, dist, row4BucketCache("acme-cn-assets"))
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != "acme-cn-assets" {
		t.Fatalf("ResourceIDs = %v, want [acme-cn-assets]", ids)
	}
}

// TestS3_0916_Dev_Row4_BareEndpointAddressesNoBucket pins the empty-name case:
// a host that is only the endpoint names no bucket, and reading one as a
// bucket called "" matches whatever cache row came back without an id.
func TestS3_0916_Dev_Row4_BareEndpointAddressesNoBucket(t *testing.T) {
	dist := row4Distribution("s3.us-east-1.amazonaws.com")
	got := checkerByTarget(t, "cf", "s3")(context.Background(), nil, dist, row4BucketCache("", "my-assets"))
	if got.Count() != 0 {
		t.Fatalf("Count = %d (%v), want 0 — a bare endpoint addresses no bucket", got.Count(), got.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// Row 2 — the empty list and the element that is not an ARN
// ---------------------------------------------------------------------------

// TestS3_0916_Dev_Row2_EmptyAndMalformedElements pins that an element which is
// not a usable ARN drops out rather than becoming a row that navigates
// nowhere, while the usable elements beside it still report.
func TestS3_0916_Dev_Row2_EmptyAndMalformedElements(t *testing.T) {
	bucket := resource.Resource{
		ID:   row2Bucket,
		Name: row2Bucket,
		Fields: map[string]string{
			"notification_lambda": "," + row2LambdaA + ",not-an-arn,",
			"notification_sqs":    "",
			"notification_sns":    "arn:aws:sns:us-east-1:123456789012:",
		},
	}
	cases := []struct {
		target string
		want   int
	}{
		{"lambda", 1},
		{"sqs", 0},
		{"sns", 0},
	}
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			got := checkerByTarget(t, "s3", c.target)(context.Background(), nil, bucket, resource.ResourceCache{})
			if got.State() != domain.RelatedResolved {
				t.Fatalf("State = %v, want RelatedResolved", got.State())
			}
			if got.Count() != c.want {
				t.Fatalf("Count = %d (%v), want %d", got.Count(), got.ResourceIDs(), c.want)
			}
		})
	}
}

// TestS3_0916_Dev_Row2_ErrorOutranksTruncation pins the precedence when a row
// carries both markers: a refusal is the stronger statement, and rendering it
// as a soft zero would hide that nothing was read at all.
func TestS3_0916_Dev_Row2_ErrorOutranksTruncation(t *testing.T) {
	bucket := resource.Resource{
		ID:   row2Bucket,
		Name: row2Bucket,
		Fields: map[string]string{
			"notification_error":     "AccessDenied: not authorized",
			"notification_truncated": "true",
		},
	}
	got := checkerByTarget(t, "s3", "lambda")(context.Background(), nil, bucket, resource.ResourceCache{})
	if got.State() != domain.RelatedError {
		t.Fatalf("State = %v, want RelatedError", got.State())
	}
}

// ---------------------------------------------------------------------------
// Row 1 — grantee shapes and an uninspectable block
// ---------------------------------------------------------------------------

// TestS3_0916_Dev_Row1_LogDeliveryGroupIsNotPublic pins the third group S3
// grants to. Log delivery writes access logs into the bucket; it reaches
// nobody on the internet, and reporting it would paint every log destination
// bucket broken.
func TestS3_0916_Dev_Row1_LogDeliveryGroupIsNotPublic(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{grants: []s3types.Grant{{
		Grantee: &s3types.Grantee{
			Type: s3types.TypeGroup,
			URI:  aws.String("http://acs.amazonaws.com/groups/s3/LogDelivery"),
		},
		Permission: s3types.PermissionWrite,
	}}})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if _, ok := row1Finding(got, row1CodePublic); ok {
		t.Errorf("findings = %v, want no %s", got.Findings[row1Bucket], row1CodePublic)
	}
}

// TestS3_0916_Dev_Row1_TwoPublicGrantsBothReported pins that a bucket granting
// two permissions publicly shows both: an operator revoking only the one named
// would leave the other live.
func TestS3_0916_Dev_Row1_TwoPublicGrantsBothReported(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{grants: []s3types.Grant{
		row1GroupGrant(row1AllUsers, s3types.PermissionRead),
		row1GroupGrant(row1AuthUser, s3types.PermissionWrite),
	}})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	n := 0
	for _, row := range row1Rows(got, row1CodePublic) {
		if row.Label == row1ACLLabel {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%q rows = %d, want 2", row1ACLLabel, n)
	}
}

// row1DeniedBlockFake answers the ACL but refuses the public access block, the
// shape of a role scoped to one of the two permissions.
type row1DeniedBlockFake struct {
	row1S3Fake
}

func (f *row1DeniedBlockFake) GetPublicAccessBlock(
	_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: s3:GetBucketPublicAccessBlock"}
}

// TestS3_0916_Dev_Row1_RefusedBlockDoesNotDisarmTheGrant pins the conservative
// read when the setting that would neutralise a grant could not be inspected:
// the grant is reported, because the alternative is a silently public bucket.
func TestS3_0916_Dev_Row1_RefusedBlockDoesNotDisarmTheGrant(t *testing.T) {
	fake := &row1DeniedBlockFake{row1S3Fake{
		grants: []s3types.Grant{row1GroupGrant(row1AllUsers, s3types.PermissionRead)},
	}}
	clients := &awsclient.ServiceClients{S3: fake, Region: "us-east-1"}
	res := []resource.Resource{{ID: row1Bucket, Name: row1Bucket, Fields: map[string]string{"name": row1Bucket}}}

	got, _ := awsclient.EnrichS3Posture(context.Background(), clients, res, nil)
	if _, ok := row1Finding(got, row1CodePublic); !ok {
		t.Fatalf("findings = %v, want %s", got.Findings[row1Bucket], row1CodePublic)
	}
}

// ---------------------------------------------------------------------------
// Row 5 — the page boundary the table does not cover
// ---------------------------------------------------------------------------

// TestS3_0916_Dev_Row5_EmptyTokenIsNotTruncation pins that a NextToken present
// but empty is the last page. Reading the field's presence rather than its
// value would mark every stack a lower bound.
func TestS3_0916_Dev_Row5_EmptyTokenIsNotTruncation(t *testing.T) {
	fake := &row5EmptyTokenFake{}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	stack := resource.Resource{ID: row5StackName, Name: row5StackName}

	got := checkerByTarget(t, "cfn", "s3")(context.Background(), clients, stack, resource.ResourceCache{})
	if got.Truncated() {
		t.Error("Truncated = true, want false")
	}
	if got.Count() != 1 {
		t.Errorf("Count = %d, want 1", got.Count())
	}
}

// row5EmptyTokenFake returns a set-but-empty NextToken.
type row5EmptyTokenFake struct {
	awsclient.CFNAPI
}

func (f *row5EmptyTokenFake) ListStackResources(
	_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options),
) (*cloudformation.ListStackResourcesOutput, error) {
	return &cloudformation.ListStackResourcesOutput{
		StackResourceSummaries: []cfntypes.StackResourceSummary{row5Summary("AWS::S3::Bucket", "acme-web-assets")},
		NextToken:              aws.String(""),
	}, nil
}

// TestS3_0916_Dev_Row5_NoStackNameIsAProvenZero pins the empty-id case: the
// pivot makes no call, and a zero it did not have to read is exact.
func TestS3_0916_Dev_Row5_NoStackNameIsAProvenZero(t *testing.T) {
	fake := &row5CFNFake{}
	clients := &awsclient.ServiceClients{CloudFormation: fake}

	got := checkerByTarget(t, "cfn", "s3")(context.Background(), clients, resource.Resource{}, resource.ResourceCache{})
	if got.Count() != 0 || got.Truncated() {
		t.Errorf("Count = %d, Truncated = %v, want 0 and false", got.Count(), got.Truncated())
	}
	if fake.calls != 0 {
		t.Errorf("ListStackResources calls = %d, want 0", fake.calls)
	}
}
