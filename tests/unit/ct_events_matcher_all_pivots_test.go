package unit_test

// ct_events_matcher_all_pivots_test.go — row 6: an id in an event body is a
// claim about the past.
//
// Eleven typed pivots route through ctEventsMatchTarget. The event names the
// resource it acted on, which is why trusting those ids looked safe: they are
// well-formed, and the call really did happen. What they do not establish is
// that the resource is still there — a DeleteBucket event names a bucket
// precisely because it no longer exists. Only the target list can turn a claim
// into a count, so every pivot is pinned through the shared helper rather than
// one of them standing in for the rest.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ctPivot is one typed pivot: the target, the AWS resource type its event
// entry carries, and the id that entry names.
type ctPivot struct {
	target  string
	awsType string
	id      string
}

func ctPivots() []ctPivot {
	return []ctPivot{
		{"ec2", "AWS::EC2::Instance", "i-0abc123def4567890"},
		{"s3", "AWS::S3::Bucket", "acme-retired-archive"},
		{"lambda", "AWS::Lambda::Function", "acme-order-processor"},
		{"dbi", "AWS::RDS::DBInstance", "acme-orders-prod"},
		{"kms", "AWS::KMS::Key", "1234abcd-12ab-34cd-56ef-1234567890ab"},
		{"secrets", "AWS::SecretsManager::Secret", "acme-prod-db-password"},
		{"sg", "AWS::EC2::SecurityGroup", "sg-0abc123def4567890"},
		{"ddb", "AWS::DynamoDB::Table", "acme-orders"},
		{"trail", "AWS::CloudTrail::Trail", "acme-management-trail"},
		{"cfn", "AWS::CloudFormation::Stack", "acme-network"},
		{"vpce", "AWS::EC2::VPCEndpoint", "vpce-0abc123def4567890"},
	}
}

// ctPivotEvent builds one event naming every pivot's resource at once, so each
// checker finds its own id and ignores the others. vpce reads its id from the
// event JSON rather than the Resources slice, so it rides there too.
func ctPivotEvent() resource.Resource {
	var res []cloudtrailtypes.Resource
	for _, p := range ctPivots() {
		res = append(res, cloudtrailtypes.Resource{
			ResourceType: aws.String(p.awsType),
			ResourceName: aws.String(p.id),
		})
	}
	return resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7b8",
		Name: "DeleteBucket",
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-0a1b2c3d4e5f6a7b8"),
			EventName: aws.String("DeleteBucket"),
			Resources: res,
			CloudTrailEvent: aws.String(
				`{"requestParameters":{"vpcEndpointId":"vpce-0abc123def4567890"},"vpcEndpointId":"vpce-0abc123def4567890"}`),
		},
	}
}

// TestCtEventsPivots_ColdCacheIsUnknown pins the case the live account was in:
// nothing cached and nothing to call, so no pivot may report a count.
func TestCtEventsPivots_ColdCacheIsUnknown(t *testing.T) {
	event := ctPivotEvent()
	for _, p := range ctPivots() {
		t.Run(p.target, func(t *testing.T) {
			restore := resource.GetPaginatedFetcher(p.target)
			resource.SetPaginatedForTest(p.target, nil)
			t.Cleanup(func() { resource.SetPaginatedForTest(p.target, restore) })

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, event, resource.ResourceCache{})

			if got := result.EffectiveState(); got != domain.RelatedUnknown {
				t.Errorf("state = %v, want RelatedUnknown: nothing read the %s list, so %q is a claim, not a count; IDs=%v",
					got, p.target, p.id, result.ResourceIDs())
			}
		})
	}
}

// TestCtEventsPivots_TruncatedConfirmsOnlyWhatItSaw pins the middle case: the
// first page names the resource, so that one id is real; the truncated flag
// rides along because a later page may name more.
func TestCtEventsPivots_TruncatedConfirmsOnlyWhatItSaw(t *testing.T) {
	event := ctPivotEvent()
	for _, p := range ctPivots() {
		t.Run(p.target, func(t *testing.T) {
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources:   []resource.Resource{{ID: p.id, Name: p.id}},
				IsTruncated: true,
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the first page confirmed %q", got, p.id)
			}
			if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != p.id {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), p.id)
			}
			if !result.Truncated() {
				t.Error("Truncated = false, want true: a later page may name more")
			}
		})
	}
}

// TestCtEventsPivots_TruncatedConfirmingNothingIsALowerBound pins the third
// case: a truncated list that confirmed nothing is a resolved zero carrying
// the truncation flag, rendered "(0+)". Unknown belongs to the case above,
// where nothing was read.
func TestCtEventsPivots_TruncatedConfirmingNothingIsALowerBound(t *testing.T) {
	event := ctPivotEvent()
	for _, p := range ctPivots() {
		t.Run(p.target, func(t *testing.T) {
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources:   []resource.Resource{{ID: "unrelated-" + p.target, Name: "unrelated-" + p.target}},
				IsTruncated: true,
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Errorf("state = %v, want RelatedResolved: the list was read; IDs=%v",
					got, result.ResourceIDs())
			}
			if !result.Truncated() {
				t.Error("Truncated = false, want true (a later page may still confirm)")
			}
		})
	}
}

// TestCtEventsPivots_ProvenListResolvesZero is the counterpart that stops the
// fix from being "always Unknown": a complete list that does not name the
// resource proves it is gone, which is what a DeleteBucket event should show.
func TestCtEventsPivots_ProvenListResolvesZero(t *testing.T) {
	event := ctPivotEvent()
	for _, p := range ctPivots() {
		t.Run(p.target, func(t *testing.T) {
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: "unrelated-" + p.target, Name: "unrelated-" + p.target}},
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the list is complete", got)
			}
			if result.Count() != 0 {
				t.Errorf("Count = %d, want 0; IDs = %v", result.Count(), result.ResourceIDs())
			}
		})
	}
}

// TestCtEventsPivots_DemoBenchDeletedBucketWitness is the bench witness for
// row 6. CtEventDeletedBucket is a DeleteBucket call naming acme-retired-archive,
// which no demo S3 fixture carries — exactly what a real DeleteBucket leaves
// behind. Its S3 panel must read a resolved zero rather than offering the
// deleted bucket as a row. The sibling CreateBucket event names a bucket that
// does exist, so a fix that merely zeroed the pivot fails here.
func TestCtEventsPivots_DemoBenchDeletedBucketWitness(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	events := byType["ct-events"]
	if len(events) == 0 {
		t.Fatal("no demo ct-events fixtures — the witness cannot be seen")
	}
	s3Rows := byType["s3"]
	if len(s3Rows) == 0 {
		t.Fatal("no demo s3 fixtures — a resolved zero would be vacuous")
	}
	cache := resource.ResourceCache{"s3": resource.ResourceCacheEntry{Resources: s3Rows}}
	checker := ctEventsCheckerByTarget(t, "s3")

	byID := make(map[string]resource.Resource, len(events))
	for _, e := range events {
		byID[e.ID] = e
	}

	for _, tc := range []struct {
		id   string
		what string
		want int
	}{
		{"evt-0a1b2c3d4e5f60009", "DeleteBucket names a bucket that is gone", 0},
		{"evt-0a1b2c3d4e5f60001", "CreateBucket names a bucket that exists", 1},
	} {
		t.Run(tc.what, func(t *testing.T) {
			event, ok := byID[tc.id]
			if !ok {
				t.Fatalf("demo fixture %s missing", tc.id)
			}
			result := checker(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the demo s3 list is complete", got)
			}
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d; IDs = %v", result.Count(), tc.want, result.ResourceIDs())
			}
		})
	}
}
