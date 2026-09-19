package unit

// A successful fetch of an empty target population is a proven zero:
// FetchRelatedTarget returns a non-nil length-0 slice, because reverse-scan
// checkers read nil as "the fetch never ran" (UnknownRelated).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

// With no fetcher registered the fetch never ran, so nil (unknown) is the
// correct answer.
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
