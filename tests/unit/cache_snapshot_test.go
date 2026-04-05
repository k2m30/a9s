package unit

// cache_snapshot_test.go — Tests that buildResourceCacheSnapshot correctly
// propagates IsTruncated from the internal resourceCacheEntry to the exported
// ResourceCacheEntry used by related checkers.
//
// Phase 1 (#218): ResourceCache type change.

import (
	"context"
	"testing"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/aws-sdk-go-v2/aws"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestBuildResourceCacheSnapshot_IncludesTruncation verifies that when
// IsTruncated=true is set in a ResourceCacheEntry passed to a related checker,
// the checker returns Count=-1 (unknown) when 0 local matches are found,
// rather than Count=0 (definitely none).
//
// This test covers the IsTruncated propagation path from cache → checker.
func TestBuildResourceCacheSnapshot_IncludesTruncation(t *testing.T) {
	instance := resource.Resource{
		ID: "i-snap-test",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-snap-test"),
			VpcId:      aws.String("vpc-snap"),
		},
	}

	// Truncated alarm cache: 1 page loaded, more exist. No alarms for i-snap-test.
	truncatedCache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "alarm-for-other",
					RawStruct: cwtypes.MetricAlarm{
						AlarmName:  aws.String("alarm-for-other"),
						Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-other")}},
					},
				},
			},
			IsTruncated: true,
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, truncatedCache)

	if result.Count != -1 {
		t.Errorf("alarm checker with truncated cache (IsTruncated=true) and 0 local matches: want Count=-1, got Count=%d", result.Count)
	}
}

// TestBuildResourceCacheSnapshot_TruncatedWithMatch_ReturnsMatches verifies that
// when IsTruncated=true but local matches ARE found, the checker returns those
// matches (not -1). Truncation only upgrades "0 matches" → unknown.
func TestBuildResourceCacheSnapshot_TruncatedWithMatch_ReturnsMatches(t *testing.T) {
	instance := resource.Resource{
		ID: "i-has-alarm",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-has-alarm"),
			VpcId:      aws.String("vpc-snap"),
		},
	}

	// Truncated cache with an alarm that DOES match this instance.
	truncatedCacheWithMatch := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "alarm-match",
					RawStruct: cwtypes.MetricAlarm{
						AlarmName:  aws.String("alarm-match"),
						Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-has-alarm")}},
					},
				},
			},
			IsTruncated: true,
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, instance, truncatedCacheWithMatch)

	// Found a match — count must be positive even though cache is truncated.
	if result.Count < 1 {
		t.Errorf("alarm checker with truncated cache and 1 local match: want Count>=1, got Count=%d", result.Count)
	}
}
