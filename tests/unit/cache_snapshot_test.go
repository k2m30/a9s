package unit

import (
	"context"
	"testing"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/aws-sdk-go-v2/aws"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A truncated cache with no local match is a lower bound, not a definitive zero.
func TestBuildResourceCacheSnapshot_IncludesTruncation(t *testing.T) {
	instance := resource.Resource{
		ID: "i-snap-test",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-snap-test"),
			VpcId:      aws.String("vpc-snap"),
		},
	}

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

	if result.Count() != 0 {
		t.Errorf("alarm checker with truncated cache (IsTruncated=true) and 0 local matches: want Count=0, got Count=%d", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("alarm checker with truncated cache (IsTruncated=true) and 0 local matches: want Truncated=true, got false")
	}
}

// Truncation affects only a zero-match answer; local matches are returned as found.
func TestBuildResourceCacheSnapshot_TruncatedWithMatch_ReturnsMatches(t *testing.T) {
	instance := resource.Resource{
		ID: "i-has-alarm",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-has-alarm"),
			VpcId:      aws.String("vpc-snap"),
		},
	}

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

	if result.Count() < 1 {
		t.Errorf("alarm checker with truncated cache and 1 local match: want Count>=1, got Count=%d", result.Count())
	}
}
