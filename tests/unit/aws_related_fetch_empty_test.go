package unit

// aws_related_fetch_empty_test.go — pins the FIX 1 contract: a successfully
// fetched EMPTY target population must render "(0)", not a blank/unknown row.
//
// FetchRelatedTarget (core/aws/related_fetch.go:27), on a cache miss with
// a registered paginated fetcher, must turn a successful zero-result fetch
// into a non-nil length-0 slice — never the bare nil that reverse-scan
// checkers interpret as "the fetch never ran" (UnknownRelated). The no-fetcher
// path (nil, unchanged) is covered as a control.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestFetchRelatedTarget_CacheMiss_FetcherSucceeds_ZeroResults verifies that
// when the registered paginated fetcher runs successfully and finds zero
// resources, FetchRelatedTarget returns a non-nil length-0 slice (so callers
// can render a proven-zero "(0)" instead of a blank/unknown row).
func TestFetchRelatedTarget_CacheMiss_FetcherSucceeds_ZeroResults(t *testing.T) {
	const target = "test-empty-fetch-target"
	resource.SetPaginatedForTest(target, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  nil,
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})
	t.Cleanup(func() {
		resource.CleanupPaginatedForTest(target)
	})

	resources, truncated, err := awsclient.FetchRelatedTarget(context.Background(), nil, resource.ResourceCache{}, target)

	if err != nil {
		t.Fatalf("FetchRelatedTarget successful empty fetch: unexpected error: %v", err)
	}
	if truncated {
		t.Errorf("FetchRelatedTarget successful empty fetch: IsTruncated = true, want false")
	}
	if resources == nil {
		t.Errorf("FetchRelatedTarget successful empty fetch: got nil slice, want non-nil length-0 slice " +
			"(a successful fetch that found 0 resources must be distinguishable from \"no fetcher ran\")")
	}
	if len(resources) != 0 {
		t.Errorf("FetchRelatedTarget successful empty fetch: got %d resources, want 0", len(resources))
	}
}

// TestFetchRelatedTarget_CacheMiss_NoFetcher_StaysNil is the control for FIX 1:
// when no fetcher is registered at all for the target, FetchRelatedTarget must
// keep returning nil — the fetch never ran, so "unknown" (blank row) remains
// correct. This must stay green both before and after the FIX 1 change.
func TestFetchRelatedTarget_CacheMiss_NoFetcher_StaysNil(t *testing.T) {
	resources, truncated, err := awsclient.FetchRelatedTarget(
		context.Background(), nil, resource.ResourceCache{}, "definitely-unregistered-target-xyz",
	)

	if err != nil {
		t.Fatalf("FetchRelatedTarget no fetcher registered: unexpected error: %v", err)
	}
	if truncated {
		t.Errorf("FetchRelatedTarget no fetcher registered: IsTruncated = true, want false")
	}
	if resources != nil {
		t.Errorf("FetchRelatedTarget no fetcher registered: got %v, want nil (no fetcher ever ran)", resources)
	}
}

// TestRelated_EC2_Alarm_FetchSucceeds_ZeroAlarms_ResolvesToZero verifies the
// FIX 1 contract propagates through a real reverse-scan consumer: a running
// EC2 instance whose CloudWatch alarm fetch succeeds with zero alarms must
// render a proven-zero "(0)" (Resolved, Count 0) — not RelatedUnknown. Real
// repro: an AWS account with 0 CloudWatch alarms currently shows a blank
// "CloudWatch Alarms" row instead of "(0)".
func TestRelated_EC2_Alarm_FetchSucceeds_ZeroAlarms_ResolvesToZero(t *testing.T) {
	mockFetcher := resource.PaginatedFetcher(func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources:  nil,
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	original := resource.GetPaginatedFetcher("alarm")
	resource.SetPaginatedForTest("alarm", mockFetcher)
	t.Cleanup(func() {
		if original != nil {
			resource.SetPaginatedForTest("alarm", original)
		} else {
			resource.CleanupPaginatedForTest("alarm")
		}
	})

	instance := resource.Resource{
		ID: "i-zero-alarms",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-zero-alarms"),
			State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if result.State() != domain.RelatedResolved {
		t.Errorf("checkEC2Alarms with 0-alarm successful fetch: State = %v, want RelatedResolved (renders \"(0)\")", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("checkEC2Alarms with 0-alarm successful fetch: Count = %d, want 0", result.Count())
	}
}
