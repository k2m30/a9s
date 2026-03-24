package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["trail"] = cloudtrailFixtures
}

// cloudtrailFixtures returns demo CloudTrail trail fixtures.
// Field keys: trail_name, s3_bucket, home_region, multi_region
func cloudtrailFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-management-trail",
			Name:   "acme-management-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "acme-management-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "true",
				"org_trail":      "false",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("acme-management-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(true),
				IsOrganizationTrail:        aws.Bool(false),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(true),
				HasCustomEventSelectors:    aws.Bool(true),
				HasInsightSelectors:        aws.Bool(false),
				CloudWatchLogsLogGroupArn:  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
			},
		},
		{
			ID:     "data-events-trail",
			Name:   "data-events-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "data-events-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "false",
				"org_trail":      "false",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("data-events-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				S3KeyPrefix:                aws.String("data-events"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(false),
				IsOrganizationTrail:        aws.Bool(false),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(false),
				HasCustomEventSelectors:    aws.Bool(true),
				HasInsightSelectors:        aws.Bool(false),
			},
		},
		{
			ID:     "security-audit-trail",
			Name:   "security-audit-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "security-audit-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/security-audit-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "true",
				"org_trail":      "true",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("security-audit-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/security-audit-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				S3KeyPrefix:                aws.String("security"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(true),
				IsOrganizationTrail:        aws.Bool(true),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(true),
				HasCustomEventSelectors:    aws.Bool(false),
				HasInsightSelectors:        aws.Bool(true),
				CloudWatchLogsLogGroupArn:  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
			},
		},
	}
}
