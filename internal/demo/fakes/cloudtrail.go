// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CloudTrailFake implements aws.CloudTrailAPI against fixture data loaded at construction time.
type CloudTrailFake struct {
	fix *fixtures.CloudTrailFixtures
}

// NewCloudTrail constructs a CloudTrailFake backed by fixture data from the fixtures package.
func NewCloudTrail() *CloudTrailFake {
	return &CloudTrailFake{fix: fixtures.NewCloudTrailFixtures()}
}

func (f *CloudTrailFake) DescribeTrails(_ context.Context, _ *cloudtrail.DescribeTrailsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{TrailList: f.fix.Trails}, nil
}

func (f *CloudTrailFake) LookupEvents(_ context.Context, input *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	if len(input.LookupAttributes) == 0 {
		return &cloudtrail.LookupEventsOutput{Events: f.fix.Events}, nil
	}
	var filtered []cloudtrailtypes.Event
	for _, evt := range f.fix.Events {
		if matchesLookupAttributes(evt, input.LookupAttributes) {
			filtered = append(filtered, evt)
		}
	}
	return &cloudtrail.LookupEventsOutput{Events: filtered}, nil
}

// matchesLookupAttributes returns true if the event matches ALL provided attributes (AND logic).
func matchesLookupAttributes(evt cloudtrailtypes.Event, attrs []cloudtrailtypes.LookupAttribute) bool {
	for _, attr := range attrs {
		val := aws.ToString(attr.AttributeValue)
		switch attr.AttributeKey {
		case cloudtrailtypes.LookupAttributeKeyUsername:
			if aws.ToString(evt.Username) != val {
				return false
			}
		case cloudtrailtypes.LookupAttributeKeyEventName:
			if aws.ToString(evt.EventName) != val {
				return false
			}
		case cloudtrailtypes.LookupAttributeKeyEventSource:
			if aws.ToString(evt.EventSource) != val {
				return false
			}
		case cloudtrailtypes.LookupAttributeKeyResourceName:
			found := false
			for _, r := range evt.Resources {
				if aws.ToString(r.ResourceName) == val {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case cloudtrailtypes.LookupAttributeKeyResourceType:
			found := false
			for _, r := range evt.Resources {
				if aws.ToString(r.ResourceType) == val {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case cloudtrailtypes.LookupAttributeKeyAccessKeyId:
			if aws.ToString(evt.AccessKeyId) != val {
				return false
			}
		}
	}
	return true
}
