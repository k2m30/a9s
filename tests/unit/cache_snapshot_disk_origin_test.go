package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestBuildResourceCacheSnapshot_MarksDiskSeededRowsFieldsOnly: rows restored
// from the disk cache carry Fields only, and a checker that matches on
// RawStruct (checkELBCF, checkApigwCF, checkS3CFN, ...) would scan them all as
// non-matching and resolve to zero. The snapshot keeps the entry — Wave-2
// enrichers that match on Fields still need it — and marks it FieldsOnly.
// Probe rows and lazy-only Partial rows (whose Origin is the zero value,
// OriginDisk) came from a live call and are not marked.
func TestBuildResourceCacheSnapshot_MarksDiskSeededRowsFieldsOnly(t *testing.T) {
	c, s := newRuntimeCore(t)
	page := &resource.PaginationMeta{IsTruncated: false}
	s.RowStore.Observe("cf", []resource.Resource{{ID: "E1", Type: "cf", Fields: map[string]string{"domain": "d1.cloudfront.net"}}}, page, session.OriginDisk, false)
	s.RowStore.Observe("elb", []resource.Resource{{ID: "lb-1", Type: "elb"}}, page, session.OriginProbe, false)
	s.RowStore.ObservePartial("s3", []resource.Resource{{ID: "bucket-lazy", Type: "s3"}})

	snap := c.BuildResourceCacheSnapshot()
	cf, ok := snap["cf"]
	if !ok || !cf.FieldsOnly || len(cf.Resources) != 1 {
		t.Errorf("snapshot[cf] = %+v (present=%t), want the disk-seeded row kept and FieldsOnly", cf, ok)
	}
	if e, ok := snap["elb"]; !ok || e.FieldsOnly {
		t.Errorf("snapshot[elb] = %+v (present=%t), want the probe-origin row present and not FieldsOnly", e, ok)
	}
	if e, ok := snap["s3"]; !ok || e.FieldsOnly {
		t.Errorf("snapshot[s3] = %+v (present=%t), want the lazy-only row present and not FieldsOnly", e, ok)
	}
}

// TestFetchRelatedTarget_FieldsOnlyEntry_IsNotACacheHit: the related-target
// reader serves RawStruct-matching checkers, so a FieldsOnly entry is a miss
// for it. With no fetcher registered for the type the answer is unknown (nil),
// never the Fields-only rows, which would match nothing and read as zero.
func TestFetchRelatedTarget_FieldsOnlyEntry_IsNotACacheHit(t *testing.T) {
	cache := resource.ResourceCache{
		"zz-disk-only": resource.ResourceCacheEntry{
			Resources:  []resource.Resource{{ID: "row-1"}},
			FieldsOnly: true,
		},
	}
	rows, truncated, err := awsclient.FetchRelatedTarget(context.Background(), nil, cache, "zz-disk-only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != nil || truncated {
		t.Errorf("FetchRelatedTarget served the FieldsOnly entry: rows=%v truncated=%t, want nil (unknown)", rows, truncated)
	}
}
