package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t571ZoneID = "/hostedzone/Z0EXAMPLE571"
	t571Bucket = "static.example.com"
)

// t571ZoneFake serves one hosted zone whose record sets span pages the way
// Route 53 pages them: IsTruncated plus NextRecordName/NextRecordType, echoed
// back as StartRecordName/StartRecordType. failOnPage makes the call for that
// page (0-based) fail; -1 means none fails.
type t571ZoneFake struct {
	pages      [][]r53types.ResourceRecordSet
	failOnPage int
}

func (f *t571ZoneFake) ListHostedZones(context.Context, *route53.ListHostedZonesInput, ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return &route53.ListHostedZonesOutput{HostedZones: []r53types.HostedZone{{
		Id:                     aws.String(t571ZoneID),
		Name:                   aws.String("example.com."),
		CallerReference:        aws.String("t571-ref"),
		ResourceRecordSetCount: aws.Int64(6),
		Config:                 &r53types.HostedZoneConfig{PrivateZone: false},
	}}}, nil
}

func (f *t571ZoneFake) ListResourceRecordSets(_ context.Context, in *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	page := 0
	if start := aws.ToString(in.StartRecordName); start != "" {
		page = -1
		for i := 1; i < len(f.pages); i++ {
			if aws.ToString(f.pages[i][0].Name) == start && f.pages[i][0].Type == in.StartRecordType {
				page = i
				break
			}
		}
		if page < 0 {
			return nil, errors.New("InvalidInput: StartRecordName does not continue a page")
		}
	}
	if page == f.failOnPage {
		return nil, errors.New("AccessDenied: route53:ListResourceRecordSets denied")
	}
	out := &route53.ListResourceRecordSetsOutput{ResourceRecordSets: f.pages[page], MaxItems: aws.Int32(3)}
	if page+1 < len(f.pages) {
		next := f.pages[page+1][0]
		out.IsTruncated = true
		out.NextRecordName = next.Name
		out.NextRecordType = next.Type
	}
	return out, nil
}

func t571Record(name string, typ r53types.RRType) r53types.ResourceRecordSet {
	return r53types.ResourceRecordSet{Name: aws.String(name), Type: typ, TTL: aws.Int64(300),
		ResourceRecords: []r53types.ResourceRecord{{Value: aws.String("192.0.2.10")}}}
}

func t571WebsiteAlias(name string) r53types.ResourceRecordSet {
	return r53types.ResourceRecordSet{Name: aws.String(name), Type: r53types.RRTypeA, AliasTarget: &r53types.AliasTarget{
		DNSName:      aws.String("s3-website-us-east-1.amazonaws.com."),
		HostedZoneId: aws.String("Z3AQBSTGFYJSTF"),
	}}
}

// Page one holds only ordinary records; the bucket's website alias is on page two.
func t571TwoPageZone(failOnPage int) *t571ZoneFake {
	return &t571ZoneFake{
		failOnPage: failOnPage,
		pages: [][]r53types.ResourceRecordSet{
			{t571Record("example.com.", r53types.RRTypeNs), t571Record("example.com.", r53types.RRTypeSoa), t571Record("api.example.com.", r53types.RRTypeA)},
			{t571WebsiteAlias(t571Bucket + "."), t571Record("www.example.com.", r53types.RRTypeA)},
		},
	}
}

// The zone row as the fetcher builds it, then the bucket's s3 -> r53 pivot
// against a cache holding that row.
func t571S3R53(t *testing.T, fake *t571ZoneFake) resource.RelatedCheckResult {
	t.Helper()
	res, _ := awsclient.FetchHostedZonesPage(context.Background(), fake, "") //nolint:errcheck // a per-zone record-scan failure is reported beside the row; the pivot must read it off the row
	if len(res.Resources) != 1 {
		t.Fatalf("zone rows = %d, want 1 — a zone whose record scan failed is still a zone", len(res.Resources))
	}
	cache := resource.ResourceCache{"r53": resource.ResourceCacheEntry{Resources: res.Resources}}
	bucket := resource.Resource{ID: t571Bucket, Name: t571Bucket}
	return checkerByTarget(t, "s3", "r53")(context.Background(), nil, bucket, cache)
}

// The website alias on the zone's second record page is found: the scan
// follows NextRecordName/NextRecordType through the zone.
func TestT571_S3R53_AliasOnSecondRecordPageIsFound(t *testing.T) {
	got := t571S3R53(t, t571TwoPageZone(-1))
	if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != t571ZoneID {
		t.Fatalf("ResourceIDs = %v (truncated %v), want [%s]", ids, got.Truncated(), t571ZoneID)
	}
	if got.Truncated() {
		t.Error("Truncated = true, want false — the whole zone was read")
	}
}

// The scan fails on page two, where the alias is: the zone was not read
// whole, so the bucket's Route 53 row is a lower bound or unknown.
func TestT571_S3R53_RecordScanFailingPartWayIsNotAConfidentNonMatch(t *testing.T) {
	got := t571S3R53(t, t571TwoPageZone(1))
	if got.State() == domain.RelatedResolved && !got.Truncated() {
		t.Errorf("state=%v count=%d truncated=false: a zone whose record scan failed part-way reads as a confident non-match",
			got.State(), got.Count())
	}
}

// The scan fails on its first call: nothing of the zone was read.
func TestT571_S3R53_RecordScanFailingOutrightIsNotAConfidentNonMatch(t *testing.T) {
	got := t571S3R53(t, t571TwoPageZone(0))
	if got.State() == domain.RelatedResolved && !got.Truncated() {
		t.Errorf("state=%v count=%d truncated=false: a zone whose record scan failed reads as a confident non-match",
			got.State(), got.Count())
	}
}

// A zone read whole with no alias to the bucket is an exact zero.
func TestT571_S3R53_WholeZoneWithoutAliasIsAnExactZero(t *testing.T) {
	fake := t571TwoPageZone(-1)
	fake.pages[1][0] = t571Record("static-old.example.com.", r53types.RRTypeCname)
	got := t571S3R53(t, fake)
	if got.State() != domain.RelatedResolved || got.Count() != 0 || got.Truncated() {
		t.Errorf("state=%v count=%d truncated=%v, want resolved exact 0", got.State(), got.Count(), got.Truncated())
	}
}
