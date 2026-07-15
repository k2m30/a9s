package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
)

// CloudTrailDescribeTrailsAPI defines the interface for the CloudTrail DescribeTrails operation.
type CloudTrailDescribeTrailsAPI interface {
	DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
	GetTrailStatus(ctx context.Context, params *cloudtrail.GetTrailStatusInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error)
}

// CloudTrailLookupEventsAPI defines the interface for the CloudTrail LookupEvents operation.
type CloudTrailLookupEventsAPI interface {
	LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

// CloudTrailAPI is the aggregate interface covering all CloudTrail operations used by a9s fetchers.
// *cloudtrail.Client structurally satisfies this interface.
type CloudTrailAPI interface {
	CloudTrailDescribeTrailsAPI
	CloudTrailLookupEventsAPI
}
