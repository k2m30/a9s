package unit_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t563CFPagesFake answers ListDistributionsByWebACLId as CloudFront does:
// at most pageSize distributions per call, the rest reached through
// Marker/NextMarker.
type t563CFPagesFake struct {
	fakeCloudFrontAPI
	ids      []string
	pageSize int
	markers  []string
}

func (f *t563CFPagesFake) ListDistributionsByWebACLId(_ context.Context, in *cloudfront.ListDistributionsByWebACLIdInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
	f.markers = append(f.markers, aws.ToString(in.Marker))
	start := 0
	if in.Marker != nil {
		start, _ = strconv.Atoi(*in.Marker) //nolint:errcheck // the fake only ever hands out numeric markers
	}
	end := min(start+f.pageSize, len(f.ids))
	list := &cftypes.DistributionList{IsTruncated: aws.Bool(end < len(f.ids))}
	for _, id := range f.ids[start:end] {
		list.Items = append(list.Items, cftypes.DistributionSummary{Id: aws.String(id)})
	}
	if end < len(f.ids) {
		list.NextMarker = aws.String(strconv.Itoa(end))
	}
	return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: list}, nil
}

func t563CFScopeACL() resource.Resource {
	arn := "arn:aws:wafv2:us-east-1:123456789012:global/webacl/my-cf-waf/a1b2c3d4-5678-90ab-cdef-222222222222"
	return resource.Resource{ID: arn, Name: "my-cf-waf", Fields: map[string]string{"name": "my-cf-waf", "arn": arn, "scope": "CLOUDFRONT"}}
}

func t563DistIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("E%013d", i)
	}
	return ids
}

// A web ACL's distributions past the first page are reached through
// NextMarker, and the pivot counts every one of them exactly.
func TestT563_WAFCloudFrontPivotFollowsNextMarker(t *testing.T) {
	fake := &t563CFPagesFake{ids: t563DistIDs(250), pageSize: 100}
	result := wafCheckerByTarget(t, "cf")(context.Background(), &awsclient.ServiceClients{CloudFront: fake}, t563CFScopeACL(), nil)
	if result.Err() != nil {
		t.Fatalf("Err = %v", result.Err())
	}
	if got := result.ResourceIDs(); !slices.Equal(got, fake.ids) {
		t.Errorf("ResourceIDs has %d ids, want all 250", len(got))
	}
	if result.Truncated() {
		t.Error("Truncated = true after the last page, want an exact count")
	}
	if want := []string{"", "100", "200"}; !slices.Equal(fake.markers, want) {
		t.Errorf("Markers sent = %q, want %q", fake.markers, want)
	}
}

// Past the per-parent page cap the answer is a lower bound, never an exact count.
func TestT563_WAFCloudFrontPivotPastThePageCapIsALowerBound(t *testing.T) {
	fake := &t563CFPagesFake{ids: t563DistIDs(awsclient.PerParentPageCap*2 + 1), pageSize: 2}
	result := wafCheckerByTarget(t, "cf")(context.Background(), &awsclient.ServiceClients{CloudFront: fake}, t563CFScopeACL(), nil)
	if result.Err() != nil {
		t.Fatalf("Err = %v", result.Err())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false with %d of %d distributions read, want a lower bound", len(result.ResourceIDs()), len(fake.ids))
	}
	if len(fake.markers) != awsclient.PerParentPageCap {
		t.Errorf("pages read = %d, want the cap %d", len(fake.markers), awsclient.PerParentPageCap)
	}
}

// The demo CloudFront client pages ListDistributionsByWebACLId as AWS does:
// MaxItems bounds a page, NextMarker names where the next one starts, and a
// Marker past the last distribution answers none.
func TestT563Demo_CloudFrontPagesDistributionsByWebACL(t *testing.T) {
	api := demo.NewServiceClients().CloudFront.(awsclient.CloudFrontListDistributionsByWebACLIdAPI)
	list := func(maxItems *int32, marker *string) *cftypes.DistributionList {
		t.Helper()
		out, err := api.ListDistributionsByWebACLId(context.Background(), &cloudfront.ListDistributionsByWebACLIdInput{WebACLId: aws.String(fixtures.WAFCloudFrontACLArn), MaxItems: maxItems, Marker: marker})
		if err != nil {
			t.Fatalf("ListDistributionsByWebACLId: %v", err)
		}
		return out.DistributionList
	}
	all := list(nil, nil)
	n := len(all.Items)
	if n == 0 {
		t.Fatal("the demo CLOUDFRONT-scope web ACL protects no distribution")
	}
	if all.NextMarker != nil {
		t.Errorf("a %d-distribution answer under the default MaxItems carries NextMarker %q", n, *all.NextMarker)
	}
	var paged []string
	var marker *string
	for range n + 1 {
		page := list(aws.Int32(1), marker)
		if len(page.Items) > 1 {
			t.Fatalf("MaxItems=1 answered %d distributions", len(page.Items))
		}
		for _, d := range page.Items {
			paged = append(paged, aws.ToString(d.Id))
		}
		if marker = page.NextMarker; aws.ToString(marker) == "" {
			break
		}
	}
	var want []string
	for _, d := range all.Items {
		want = append(want, aws.ToString(d.Id))
	}
	if !slices.Equal(paged, want) {
		t.Errorf("paged by one = %v, want %v", paged, want)
	}
	if past := list(nil, aws.String(strconv.Itoa(n))); len(past.Items) != 0 {
		t.Errorf("Marker past the last distribution answered %d", len(past.Items))
	}
}
