package unit_test

// s3_0916_codex_round3_test.go pins the three findings the external review
// raised against the landing range, all of them the same shape: a pivot
// reporting an exact figure it did not earn, or a sentence claiming more than
// the data says.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// lambda → s3 carries the bucket list's own incompleteness
// ---------------------------------------------------------------------------

// r3Bucket builds a bucket cache row with the given notification fields.
func r3Bucket(id string, fields map[string]string) resource.Resource {
	return resource.Resource{ID: id, Name: id, Fields: fields}
}

func r3LambdaS3(t *testing.T, buckets ...resource.Resource) resource.RelatedCheckResult {
	t.Helper()
	cache := resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: buckets}}
	return checkerByTarget(t, "lambda", "s3")(
		context.Background(), nil, row2LambdaResource("acme-notify-a", row2LambdaA), cache)
}

// TestS3_0916_R3_LambdaS3_RefusedBucketMakesTheResultALowerBound pins that a
// bucket whose notification lookup was refused makes lambda → s3 a lower
// bound. That bucket may notify this function; nobody knows, and an exact
// count says nobody has to check.
func TestS3_0916_R3_LambdaS3_RefusedBucketMakesTheResultALowerBound(t *testing.T) {
	got := r3LambdaS3(t,
		r3Bucket("acme-events", map[string]string{"notification_lambda": row2LambdaA}),
		r3Bucket("acme-denied", map[string]string{"notification_error": "AccessDenied: not authorized"}),
	)
	if got.Count() != 1 {
		t.Fatalf("Count = %d (%v), want 1", got.Count(), got.ResourceIDs())
	}
	if !got.Truncated() {
		t.Error("Truncated = false, want true — one bucket's destinations were never read")
	}
}

// TestS3_0916_R3_LambdaS3_CrossRegionBucketMakesTheResultALowerBound is the
// same rule for the operational half: a bucket this client's region cannot
// answer for is unknown, not empty.
func TestS3_0916_R3_LambdaS3_CrossRegionBucketMakesTheResultALowerBound(t *testing.T) {
	got := r3LambdaS3(t,
		r3Bucket("acme-events", map[string]string{"notification_lambda": row2LambdaA}),
		r3Bucket("acme-eu", map[string]string{"notification_truncated": "true"}),
	)
	if got.Count() != 1 {
		t.Fatalf("Count = %d (%v), want 1", got.Count(), got.ResourceIDs())
	}
	if !got.Truncated() {
		t.Error("Truncated = false, want true — one bucket was not inspected from this region")
	}
}

// TestS3_0916_R3_LambdaS3_AllBucketsAnsweredIsExact is the negative: with
// every bucket answered, the count is a fact and must not be softened.
func TestS3_0916_R3_LambdaS3_AllBucketsAnsweredIsExact(t *testing.T) {
	got := r3LambdaS3(t,
		r3Bucket("acme-events", map[string]string{"notification_lambda": row2LambdaA}),
		r3Bucket("acme-archive", map[string]string{"notification_lambda": row2LambdaB}),
	)
	if got.Count() != 1 {
		t.Fatalf("Count = %d (%v), want 1", got.Count(), got.ResourceIDs())
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false — every bucket answered")
	}
}

// ---------------------------------------------------------------------------
// s3 → r53 carries the zone's unread record pages
// ---------------------------------------------------------------------------

// r3ZoneFake answers one hosted zone and one page of its records, with the
// page's truncation flag under the caller's control.
type r3ZoneFake struct {
	awsclient.Route53API
	sets      []r53types.ResourceRecordSet
	truncated bool
}

func (f *r3ZoneFake) ListHostedZones(
	_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options),
) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{
		HostedZones: []r53types.HostedZone{{
			Id:                     aws.String(row3ZoneID),
			Name:                   aws.String("example.com."),
			ResourceRecordSetCount: aws.Int64(400),
			Config:                 &r53types.HostedZoneConfig{PrivateZone: false},
		}},
	}, nil
}

func (f *r3ZoneFake) ListResourceRecordSets(
	_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: f.sets,
		IsTruncated:        f.truncated,
	}, nil
}

func r3FetchZone(t *testing.T, fake *r3ZoneFake) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchHostedZonesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("fetch error = %v, want nil", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("Resources = %d, want 1 zone", len(res.Resources))
	}
	return res.Resources[0]
}

// TestS3_0916_R3_ZoneRowCarriesItsUnreadRecordPages pins the fetcher
// persisting the record page's truncation. The pivot that joins on the zone's
// alias names cannot otherwise tell "this zone has no such alias" from "the
// alias is on a page nobody read".
func TestS3_0916_R3_ZoneRowCarriesItsUnreadRecordPages(t *testing.T) {
	truncated := r3FetchZone(t, &r3ZoneFake{
		sets:      []r53types.ResourceRecordSet{row3AliasRecord("other.example.com.", "s3-website-us-east-1.amazonaws.com.")},
		truncated: true,
	})
	if got := truncated.Fields["records_truncated"]; got != "true" {
		t.Errorf("Fields[\"records_truncated\"] = %q, want %q", got, "true")
	}

	whole := r3FetchZone(t, &r3ZoneFake{
		sets: []r53types.ResourceRecordSet{row3AliasRecord(row3SiteBucket+".", "s3-website-us-east-1.amazonaws.com.")},
	})
	if got := whole.Fields["records_truncated"]; got != "" {
		t.Errorf("Fields[\"records_truncated\"] = %q, want empty — the page was the whole zone", got)
	}
}

func r3S3R53(t *testing.T, zone resource.Resource) resource.RelatedCheckResult {
	t.Helper()
	cache := resource.ResourceCache{"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zone}}}
	bucket := resource.Resource{ID: row3SiteBucket, Name: row3SiteBucket}
	return checkerByTarget(t, "s3", "r53")(context.Background(), nil, bucket, cache)
}

// TestS3_0916_R3_S3R53_UnreadPagesSoftenTheZero pins the reverse pivot reading
// that flag. r53 → s3 already renders a lower bound in this situation; the two
// directions of one relationship disagreeing is the defect.
func TestS3_0916_R3_S3R53_UnreadPagesSoftenTheZero(t *testing.T) {
	got := r3S3R53(t, resource.Resource{
		ID:     row3ZoneID,
		Name:   "example.com.",
		Fields: map[string]string{"s3website_alias_names": "other.example.com", "records_truncated": "true"},
	})
	if got.Count() != 0 {
		t.Fatalf("Count = %d, want 0", got.Count())
	}
	if !got.Truncated() {
		t.Error("Truncated = false, want true — the alias may sit on a page nobody read")
	}
}

// TestS3_0916_R3_S3R53_WholeZoneIsAnExactAnswer is the negative: a zone read
// whole reports an exact zero.
func TestS3_0916_R3_S3R53_WholeZoneIsAnExactAnswer(t *testing.T) {
	got := r3S3R53(t, resource.Resource{
		ID:     row3ZoneID,
		Name:   "example.com.",
		Fields: map[string]string{"s3website_alias_names": "other.example.com"},
	})
	if got.Count() != 0 {
		t.Fatalf("Count = %d, want 0", got.Count())
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false — the zone was read whole")
	}
}

// TestS3_0916_R3_S3R53_MatchStillReportsTheZone guards the match itself
// against the truncation change.
func TestS3_0916_R3_S3R53_MatchStillReportsTheZone(t *testing.T) {
	got := r3S3R53(t, resource.Resource{
		ID:     row3ZoneID,
		Name:   "example.com.",
		Fields: map[string]string{"s3website_alias_names": row3SiteBucket},
	})
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != row3ZoneID {
		t.Fatalf("ResourceIDs = %v, want [%s]", ids, row3ZoneID)
	}
}

// TestS3_0916_R3_CatalogDeclaresRecordsTruncated pins the field on the r53
// catalog entry: a field a pivot reads and the catalog does not declare is
// invisible to the readers that rebuild a row from its declared keys.
func TestS3_0916_R3_CatalogDeclaresRecordsTruncated(t *testing.T) {
	def := catalog.TopLevelOnly("r53")
	if def == nil {
		t.Fatal(`catalog.TopLevelOnly("r53") = nil`)
	}
	for _, k := range def.FieldKeys {
		if k == "records_truncated" {
			return
		}
	}
	t.Errorf("r53 FieldKeys = %v, want it to declare %q", def.FieldKeys, "records_truncated")
}

// ---------------------------------------------------------------------------
// the public verdict claims only what the permission grants
// ---------------------------------------------------------------------------

// TestS3_0916_R3_PublicDetailDoesNotOverclaimObjectAccess pins the wording.
// An access control list READ permits listing the bucket, and READ_ACP /
// WRITE_ACP reach the access control list alone — none of the three is object
// access, so a verdict that says the objects are readable sends the operator
// to the wrong remedy.
func TestS3_0916_R3_PublicDetailDoesNotOverclaimObjectAccess(t *testing.T) {
	detail := strings.ToLower(catalog.Detail(domain.FindingCode("s3.public")))
	if detail == "" {
		t.Fatal("catalog.Detail(s3.public) is empty")
	}
	if strings.Contains(detail, "reach this bucket's objects") {
		t.Errorf("detail = %q, want it not to claim object access — the granted permission may be listing or the access control list alone", detail)
	}
	if !strings.Contains(detail, "permission") {
		t.Errorf("detail = %q, want it to point at the permission the row shows", detail)
	}
}
