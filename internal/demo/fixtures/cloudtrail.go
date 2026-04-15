// Package fixtures provides CloudTrail fixture data for the CloudTrail fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

// CloudTrailFixtures holds all CloudTrail domain objects served by the fake.
type CloudTrailFixtures struct {
	Trails      []cloudtrailtypes.Trail
	TrailStatus map[string]cloudtrail.GetTrailStatusOutput
	Events      []cloudtrailtypes.Event
}

// NewCloudTrailFixtures builds and returns a fully-populated CloudTrailFixtures struct.
func NewCloudTrailFixtures() *CloudTrailFixtures {
	return &CloudTrailFixtures{
		Trails:      buildCTTrails(),
		TrailStatus: buildCTTrailStatus(),
		Events:      buildCTEvents(),
	}
}

// buildCTTrailStatus keys GetTrailStatus responses by trail ARN. One trail is
// intentionally not logging, one has a LatestDeliveryError, the rest healthy.
func buildCTTrailStatus() map[string]cloudtrail.GetTrailStatusOutput {
	return map[string]cloudtrail.GetTrailStatusOutput{
		"arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail": {
			IsLogging: aws.Bool(true),
		},
		"arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events-trail": {
			IsLogging:           aws.Bool(true),
			LatestDeliveryError: aws.String("AccessDenied: The S3 bucket policy denies CloudTrail writes"),
		},
		"arn:aws:cloudtrail:us-east-1:123456789012:trail/security-audit-trail": {
			IsLogging: aws.Bool(false),
		},
	}
}

// ---------------------------------------------------------------------------
// Trails (for DescribeTrails)
// ---------------------------------------------------------------------------

func buildCTTrails() []cloudtrailtypes.Trail {
	return []cloudtrailtypes.Trail{
		{
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
			KmsKeyId:                   aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
		{
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
		{
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
		{
			Name:                       aws.String("acme-audit-trail"),
			TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-audit-trail"),
			S3BucketName:               aws.String("data-pipeline-logs"),
			HomeRegion:                 aws.String("us-east-1"),
			IsMultiRegionTrail:         aws.Bool(false),
			IsOrganizationTrail:        aws.Bool(false),
			LogFileValidationEnabled:   aws.Bool(true),
			IncludeGlobalServiceEvents: aws.Bool(false),
			HasCustomEventSelectors:    aws.Bool(true),
			HasInsightSelectors:        aws.Bool(false),
		},
	}
}

// ---------------------------------------------------------------------------
// Events (for LookupEvents)
// ---------------------------------------------------------------------------

func buildCTEvents() []cloudtrailtypes.Event {
	t1 := time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC)
	t2 := time.Date(2026, 3, 28, 13, 45, 22, 0, time.UTC)
	t3 := time.Date(2026, 3, 28, 12, 10, 5, 0, time.UTC)
	t4 := time.Date(2026, 3, 28, 11, 55, 48, 0, time.UTC)
	t5 := time.Date(2026, 3, 28, 10, 20, 33, 0, time.UTC)
	t6 := time.Date(2026, 3, 28, 9, 5, 11, 0, time.UTC)
	tA := time.Date(2026, 4, 7, 14, 2, 11, 0, time.UTC)
	tB := time.Date(2026, 4, 7, 14, 7, 42, 0, time.UTC)
	tC := time.Date(2026, 4, 7, 14, 11, 3, 0, time.UTC)
	tD := time.Date(2026, 4, 7, 2, 0, 7, 0, time.UTC)
	tE := time.Date(2026, 4, 7, 3, 42, 18, 0, time.UTC)
	tF := time.Date(2026, 4, 7, 14, 20, 21, 0, time.UTC)
	tG := time.Date(2026, 4, 7, 14, 31, 55, 0, time.UTC)
	tH := time.Date(2026, 4, 7, 9, 14, 0, 0, time.UTC)
	tI := time.Date(2026, 4, 7, 14, 44, 17, 0, time.UTC)
	tJ := time.Date(2026, 4, 7, 15, 10, 5, 0, time.UTC)
	tK := time.Date(2026, 4, 7, 15, 12, 33, 0, time.UTC)
	tL := time.Date(2026, 4, 7, 15, 14, 58, 0, time.UTC)
	tM := time.Date(2026, 4, 9, 9, 12, 0, 0, time.UTC)
	tN := time.Date(2026, 4, 9, 8, 47, 30, 0, time.UTC)
	tO := time.Date(2026, 4, 9, 8, 32, 15, 0, time.UTC)
	tP := time.Date(2026, 4, 9, 7, 58, 0, 0, time.UTC)
	tQ := time.Date(2026, 4, 9, 7, 27, 45, 0, time.UTC)
	tR := time.Date(2026, 4, 9, 7, 44, 20, 0, time.UTC)
	tS := time.Date(2026, 4, 9, 7, 17, 10, 0, time.UTC)
	tT := time.Date(2026, 4, 9, 6, 37, 55, 0, time.UTC)

	return []cloudtrailtypes.Event{
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60001"),
			EventName:   aws.String("CreateBucket"),
			EventTime:   aws.Time(t1),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    nil,
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"Root","principalId":"123456789012","arn":"arn:aws:iam::123456789012:root","accountId":"123456789012","sessionContext":{"sessionCredentialFromConsole":"true","attributes":{"mfaAuthenticated":"true","creationDate":"2026-03-28T14:20:00Z"}}},"eventTime":"2026-03-28T14:30:15Z","eventSource":"s3.amazonaws.com","eventName":"CreateBucket","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.10","userAgent":"signin.amazonaws.com","requestParameters":{"bucketName":"webapp-assets-prod"},"responseElements":null,"requestID":"req-s3-create-001","eventID":"evt-0a1b2c3d4e5f60001","readOnly":false,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:s3:::webapp-assets-prod","accountId":"123456789012","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
			},
		},
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60002"),
			EventName:   aws.String("DeleteBucket"),
			EventTime:   aws.Time(t2),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("bob.smith"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE222222222","arn":"arn:aws:iam::123456789012:user/bob.smith","accountId":"123456789012","userName":"bob.smith"},"eventTime":"2026-03-28T13:45:22Z","eventSource":"s3.amazonaws.com","eventName":"DeleteBucket","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.20","userAgent":"aws-cli/2.15.0 Python/3.11.0 Darwin/23.0.0 botocore/2.0.0","requestParameters":{"bucketName":"webapp-assets-prod"},"responseElements":null,"errorCode":"AccessDenied","errorMessage":"User: arn:aws:iam::123456789012:user/bob.smith is not authorized to perform: s3:DeleteBucket","requestID":"req-s3-del-001","eventID":"evt-0a1b2c3d4e5f60002","readOnly":false,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:s3:::webapp-assets-prod","accountId":"123456789012","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
			},
		},
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60003"),
			EventName:   aws.String("DescribeInstances"),
			EventTime:   aws.Time(t3),
			EventSource: aws.String("ec2.amazonaws.com"),
			Username:    aws.String("acme-eks-node-role"),
			ReadOnly:    aws.String("true"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE111111111:i-0a1b2c3d4e5f60001","arn":"arn:aws:sts::123456789012:assumed-role/acme-eks-node-role/i-0a1b2c3d4e5f60001","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE111111111","arn":"arn:aws:iam::123456789012:role/acme-eks-node-role","accountId":"123456789012","userName":"acme-eks-node-role"},"sessionCredentialFromConsole":"true","attributes":{"mfaAuthenticated":"false","creationDate":"2026-03-28T12:00:00Z"}}},"eventTime":"2026-03-28T12:10:05Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.30","userAgent":"signin.amazonaws.com","requestParameters":{},"responseElements":null,"requestID":"req-ec2-desc-001","eventID":"evt-0a1b2c3d4e5f60003","readOnly":true,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management"}`),
			Resources: []cloudtrailtypes.Resource{},
		},
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60004"),
			EventName:   aws.String("TerminateInstanceInAutoScalingGroup"),
			EventTime:   aws.Time(t4),
			EventSource: aws.String("autoscaling.amazonaws.com"),
			Username:    nil,
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AWSService","invokedBy":"autoscaling.amazonaws.com"},"eventTime":"2026-03-28T11:55:48Z","eventSource":"autoscaling.amazonaws.com","eventName":"TerminateInstanceInAutoScalingGroup","awsRegion":"us-east-1","sourceIPAddress":"autoscaling.amazonaws.com","requestParameters":{"instanceId":"i-0a1b2c3d4e5f60001"},"responseElements":{"instance":{"instanceId":"i-0a1b2c3d4e5f60001","currentState":{"name":"shutting-down"}}},"requestID":"req-asg-term-001","eventID":"evt-0a1b2c3d4e5f60004","readOnly":false,"eventType":"AwsServiceEvent","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:ec2:us-east-1:123456789012:instance/i-0a1b2c3d4e5f60001","accountId":"123456789012","type":"AWS::EC2::Instance"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60001")},
			},
		},
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60005"),
			EventName:   aws.String("ApiCallRateInsight"),
			EventTime:   aws.Time(t5),
			EventSource: aws.String("cloudtrail.amazonaws.com"),
			Username:    aws.String("bob.smith"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.11","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE222222222","arn":"arn:aws:iam::123456789012:user/bob.smith","accountId":"123456789012","userName":"bob.smith"},"eventTime":"2026-03-28T10:20:33Z","eventSource":"cloudtrail.amazonaws.com","eventName":"ApiCallRateInsight","awsRegion":"us-east-1","sourceIPAddress":"","userAgent":"","requestParameters":null,"responseElements":null,"requestID":"req-insight-001","eventID":"evt-0a1b2c3d4e5f60005","readOnly":false,"eventType":"AwsApiCall","managementEvent":false,"recipientAccountId":"123456789012","eventCategory":"Insight","insightDetails":{"state":"Start","insightType":"ApiCallRateInsight","insightContext":{"statistics":{"baseline":{"average":5.0},"insight":{"average":120.0}}}}}`),
			Resources:   []cloudtrailtypes.Resource{},
		},
		{
			EventId:     aws.String("evt-0a1b2c3d4e5f60006"),
			EventName:   aws.String("VpcEndpointAccess"),
			EventTime:   aws.Time(t6),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("ci-runner"),
			ReadOnly:    aws.String("true"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE666666666:ci-session","arn":"arn:aws:sts::999988887777:assumed-role/ci-runner/ci-session","accountId":"999988887777","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE666666666","arn":"arn:aws:iam::123456789012:role/ci-runner","accountId":"123456789012","userName":"ci-runner"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-03-28T09:00:00Z"}}},"eventTime":"2026-03-28T09:05:11Z","eventSource":"s3.amazonaws.com","eventName":"VpcEndpointAccess","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.50","userAgent":"aws-sdk-java/2.20 Linux/5.15 Java/17.0","requestParameters":{},"responseElements":null,"requestID":"req-vpc-ep-001","eventID":"evt-0a1b2c3d4e5f60006","readOnly":true,"eventType":"AwsApiCall","managementEvent":false,"recipientAccountId":"123456789012","eventCategory":"NetworkActivity","vpcEndpointId":"vpce-0abc123"}`),
			Resources:   []cloudtrailtypes.Resource{},
		},
		// Wireframe cases A–L
		{
			EventId:     aws.String("e-a1b2c3d4"),
			EventName:   aws.String("DescribeInstances"),
			EventTime:   aws.Time(tA),
			EventSource: aws.String("ec2.amazonaws.com"),
			Username:    aws.String("KarpenterNodeRole"),
			ReadOnly:    aws.String("true"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:02:11Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"10.0.14.221","userAgent":"aws-sdk-go-v2/1.30.3","recipientAccountId":"111111111111","eventID":"e-a1b2c3d4","readOnly":true,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/karpenter-1759","principalId":"AROAEXAMPLE:karpenter-1759","accountId":"111111111111","accessKeyId":"ASIAY44QH8DCKARPEXMP","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::111111111111:role/KarpenterNodeRole","principalId":"AROAEXAMPLE","accountId":"111111111111","userName":"KarpenterNodeRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T13:44:02Z"}}},"requestParameters":{"filterSet":{"items":[{"name":"instance-state-name","valueSet":{"items":[{"value":"running"}]}}]},"maxResults":1000},"responseElements":null}`),
			Resources:   []cloudtrailtypes.Resource{},
		},
		{
			EventId:     aws.String("e-b2c3d4e5"),
			EventName:   aws.String("TerminateInstances"),
			EventTime:   aws.Time(tB),
			EventSource: aws.String("ec2.amazonaws.com"),
			Username:    aws.String("AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:07:42Z","eventSource":"ec2.amazonaws.com","eventName":"TerminateInstances","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"eu-west-1","sourceIPAddress":"AWS Internal","userAgent":"console.amazonaws.com","recipientAccountId":"222222222222","eventID":"e-b2c3d4e5","readOnly":false,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp","principalId":"AROAEXAMPLE:alice@corp","accountId":"222222222222","accessKeyId":"ASIAZK7L9PQRSSOXEXMP","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::222222222222:role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d","principalId":"AROAEXAMPLE","accountId":"222222222222","userName":"AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"},"attributes":{"mfaAuthenticated":"true","creationDate":"2026-04-07T14:00:00Z"}}},"requestParameters":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60001"},{"instanceId":"i-0a1b2c3d4e5f60002"}]}},"responseElements":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60001","currentState":{"code":32,"name":"shutting-down"},"previousState":{"code":16,"name":"running"}},{"instanceId":"i-0a1b2c3d4e5f60002","currentState":{"code":32,"name":"shutting-down"},"previousState":{"code":16,"name":"running"}}]}}}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60001")},
				{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60002")},
			},
		},
		{
			EventId:     aws.String("e-c3d4e5f6"),
			EventName:   aws.String("PutObject"),
			EventTime:   aws.Time(tC),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("bob"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:11:03Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.42","userAgent":"aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0","recipientAccountId":"333333333333","eventID":"e-c3d4e5f6","readOnly":false,"errorCode":"AccessDenied","errorMessage":"User: arn:aws:iam::333333333333:user/bob is not authorized to perform: s3:PutObject on resource: arn:aws:s3:::webapp-assets-prod/2026/04/07/app.log because no identity-based policy allows the s3:PutObject action","userIdentity":{"type":"IAMUser","principalId":"AIDAIOSFODNN7BOB1XMP","arn":"arn:aws:iam::333333333333:user/bob","accountId":"333333333333","accessKeyId":"AKIAIOSFODNN7BOB1XMP","userName":"bob"},"requestParameters":{"bucketName":"webapp-assets-prod","key":"2026/04/07/app.log"},"responseElements":null}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
			},
		},
		{
			EventId:     aws.String("e-d4e5f6a7"),
			EventName:   aws.String("RotateKey"),
			EventTime:   aws.Time(tD),
			EventSource: aws.String("kms.amazonaws.com"),
			Username:    nil,
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T02:00:07Z","eventSource":"kms.amazonaws.com","eventName":"RotateKey","eventCategory":"Management","eventType":"AwsServiceEvent","awsRegion":"us-east-1","sourceIPAddress":"AWS Internal","recipientAccountId":"444444444444","eventID":"e-d4e5f6a7","readOnly":false,"userIdentity":{"type":"AWSService","invokedBy":"kms.amazonaws.com"},"requestParameters":{"keyId":"arn:aws:kms:us-east-1:444444444444:key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b","rotationType":"AUTOMATIC","backingKey":true},"responseElements":null}`),
			Resources:   []cloudtrailtypes.Resource{},
		},
		{
			EventId:     aws.String("e-e5f6a7b8"),
			EventName:   aws.String("PutBucketPolicy"),
			EventTime:   aws.Time(tE),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    nil,
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T03:42:18Z","eventSource":"s3.amazonaws.com","eventName":"PutBucketPolicy","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.17","userAgent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15","recipientAccountId":"555555555555","eventID":"e-e5f6a7b8","readOnly":false,"userIdentity":{"type":"Root","principalId":"555555555555","arn":"arn:aws:iam::555555555555:root","accountId":"555555555555"},"requestParameters":{"bucketName":"prod-artifacts","policy":"{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":\"*\",\"Action\":\"s3:GetObject\",\"Resource\":\"arn:aws:s3:::prod-artifacts/*\"}]}"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::prod-artifacts","accountId":"555555555555","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("prod-artifacts")},
			},
		},
		{
			EventId:     aws.String("e-f6a7b8c9"),
			EventName:   aws.String("GetObject"),
			EventTime:   aws.Time(tF),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("eks-checkout-svc-sa"),
			ReadOnly:    aws.String("true"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:20:21Z","eventSource":"s3.amazonaws.com","eventName":"GetObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"eu-west-1","sourceIPAddress":"10.42.3.18","userAgent":"aws-sdk-go-v2/1.30.3","recipientAccountId":"666666666666","eventID":"e-f6a7b8c9","readOnly":true,"vpcEndpointId":"vpce-0abc123def456","userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824","principalId":"AROAEXAMPLE:1717156821993453824","accountId":"666666666666","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::666666666666:role/eks-checkout-svc-sa","principalId":"AROAEXAMPLE","accountId":"666666666666","userName":"eks-checkout-svc-sa"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:15:00Z"}},"webIdFederationData":{"federatedProvider":"oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C"}},"requestParameters":{"bucketName":"checkout-config","key":"prod/config.json"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::checkout-config","accountId":"666666666666","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("checkout-config")},
			},
		},
		{
			EventId:     aws.String("e-a7b8c9d0"),
			EventName:   aws.String("PutObject"),
			EventTime:   aws.Time(tG),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("CiBuildRole"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:31:55Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-2","sourceIPAddress":"52.14.88.201","userAgent":"aws-cli/2.17.9","recipientAccountId":"777777777777","eventID":"e-a7b8c9d0","readOnly":false,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821","principalId":"AROAEXAMPLE:build-4821","accountId":"888888888888","accessKeyId":"ASIAQF3M2N8KCIB1XMPL","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::888888888888:role/CiBuildRole","principalId":"AROAEXAMPLE","accountId":"888888888888","userName":"CiBuildRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:25:00Z"}}},"requestParameters":{"bucketName":"shared-artifacts","key":"build-4821.tar.gz"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::shared-artifacts","accountId":"888888888888","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("shared-artifacts")},
			},
		},
		{
			EventId:     aws.String("e-b8c9d0e1"),
			EventName:   aws.String("RunInstances"),
			EventTime:   aws.Time(tH),
			EventSource: aws.String("ec2.amazonaws.com"),
			Username:    nil,
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.11","eventTime":"2026-04-07T09:14:00Z","eventSource":"ec2.amazonaws.com","eventName":"RunInstances","eventCategory":"Insight","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"","recipientAccountId":"999999999999","eventID":"e-b8c9d0e1","readOnly":false,"requestParameters":null,"responseElements":null,"insightDetails":{"state":"Start","insightType":"ApiCallRateInsight","insightContext":{"statistics":{"baseline":{"average":0.24},"insight":{"average":18.70}},"attributions":[{"attribute":"userIdentityArn","insight":["arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41"],"baseline":["arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*"]}]}}}`),
			Resources:   []cloudtrailtypes.Resource{},
		},
		{
			EventId:     aws.String("e-c9d0e1f2"),
			EventName:   aws.String("PutObject"),
			EventTime:   aws.Time(tI),
			EventSource: aws.String("s3.amazonaws.com"),
			Username:    aws.String("DataPipelineRole"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:44:17Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"NetworkActivity","eventType":"AwsVpceEvent","awsRegion":"eu-central-1","sourceIPAddress":"10.12.4.77","userAgent":"aws-sdk-java/2.25.11","recipientAccountId":"111111111111","eventID":"e-c9d0e1f2","readOnly":false,"errorCode":"VpceAccessDenied","errorMessage":"The VPC endpoint policy denies the s3:PutObject action on arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet","vpcEndpointId":"vpce-0ff11223344556677","userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719","principalId":"AROAEXAMPLE:dp-0719","accountId":"111111111111","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::111111111111:role/DataPipelineRole","principalId":"AROAEXAMPLE","accountId":"111111111111","userName":"DataPipelineRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:40:00Z"}}},"requestParameters":{"bucketName":"prod-lake","key":"landing/2026/04/07/batch-0719.parquet"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::prod-lake","accountId":"111111111111","type":"AWS::S3::Bucket"}]}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("prod-lake")},
			},
		},
		{
			EventId:     aws.String("e-d0e1f2a3"),
			EventName:   aws.String("CreateUser"),
			EventTime:   aws.Time(tJ),
			EventSource: aws.String("iam.amazonaws.com"),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:10:05Z","eventSource":"iam.amazonaws.com","eventName":"CreateUser","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"charlie","path":"/"},"responseElements":{"user":{"userId":"AIDAIOSFODNN8EXAMPLE","arn":"arn:aws:iam::123456789012:user/charlie","path":"/","userName":"charlie","createDate":"Apr 7, 2026, 3:10:05 PM"}},"recipientAccountId":"123456789012","eventID":"e-d0e1f2a3"}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("charlie")},
			},
		},
		{
			EventId:     aws.String("e-e1f2a3b4"),
			EventName:   aws.String("AttachUserPolicy"),
			EventTime:   aws.Time(tK),
			EventSource: aws.String("iam.amazonaws.com"),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:12:33Z","eventSource":"iam.amazonaws.com","eventName":"AttachUserPolicy","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"bob","policyArn":"arn:aws:iam::aws:policy/AdministratorAccess"},"responseElements":null,"recipientAccountId":"123456789012","eventID":"e-e1f2a3b4"}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("bob")},
				{ResourceType: aws.String("AWS::IAM::ManagedPolicy"), ResourceName: aws.String("arn:aws:iam::aws:policy/AdministratorAccess")},
			},
		},
		{
			EventId:     aws.String("e-f2a3b4c5"),
			EventName:   aws.String("CreateAccessKey"),
			EventTime:   aws.Time(tL),
			EventSource: aws.String("iam.amazonaws.com"),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("false"),
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:14:58Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"bob"},"responseElements":{"accessKey":{"accessKeyId":"AKIAIOSFODNN7EXAMPLE","status":"Active","userName":"bob","createDate":"Apr 7, 2026, 3:14:58 PM"}},"recipientAccountId":"123456789012","eventID":"e-f2a3b4c5"}`),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("bob")},
			},
		},
		// Lambda events
		{
			EventId:     aws.String("evt-lambda-invoke-001"),
			EventName:   aws.String("Invoke"),
			EventSource: aws.String("lambda.amazonaws.com"),
			EventTime:   aws.Time(tM),
			Username:    aws.String("ci-service-account"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::Lambda::Function"), ResourceName: aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-orders")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","accessKeyId":"AKIAEXAMPLE002","userName":"ci-service-account"},"eventSource":"lambda.amazonaws.com","eventName":"Invoke","requestParameters":{"functionName":"arn:aws:lambda:us-east-1:123456789012:function:process-orders"}}`),
		},
		{
			EventId:     aws.String("evt-lambda-update-001"),
			EventName:   aws.String("UpdateFunctionCode20150331v2"),
			EventSource: aws.String("lambda.amazonaws.com"),
			EventTime:   aws.Time(tN),
			Username:    aws.String("ci-service-account"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::Lambda::Function"), ResourceName: aws.String("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","accessKeyId":"AKIAEXAMPLE002","userName":"ci-service-account"},"eventSource":"lambda.amazonaws.com","eventName":"UpdateFunctionCode20150331v2","requestParameters":{"functionName":"arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"}}`),
		},
		// RDS event
		{
			EventId:     aws.String("evt-rds-modify-001"),
			EventName:   aws.String("ModifyDBInstance"),
			EventSource: aws.String("rds.amazonaws.com"),
			EventTime:   aws.Time(tO),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::RDS::DBInstance"), ResourceName: aws.String("arn:aws:rds:us-east-1:123456789012:db:prod-api-primary")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","accountId":"123456789012","userName":"alice.johnson"},"eventSource":"rds.amazonaws.com","eventName":"ModifyDBInstance","requestParameters":{"dBInstanceIdentifier":"prod-api-primary"}}`),
		},
		// ECS event
		{
			EventId:     aws.String("evt-ecs-update-001"),
			EventName:   aws.String("UpdateService"),
			EventSource: aws.String("ecs.amazonaws.com"),
			EventTime:   aws.Time(tP),
			Username:    aws.String("ci-service-account"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::ECS::Cluster"), ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-services")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","userName":"ci-service-account"},"eventSource":"ecs.amazonaws.com","eventName":"UpdateService","requestParameters":{"cluster":"acme-services"}}`),
		},
		// DynamoDB event
		{
			EventId:     aws.String("evt-ddb-update-001"),
			EventName:   aws.String("UpdateTable"),
			EventSource: aws.String("dynamodb.amazonaws.com"),
			EventTime:   aws.Time(tQ),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::DynamoDB::Table"), ResourceName: aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","accountId":"123456789012","userName":"alice.johnson"},"eventSource":"dynamodb.amazonaws.com","eventName":"UpdateTable","requestParameters":{"tableName":"acme-orders"}}`),
		},
		// Secrets Manager event
		{
			EventId:     aws.String("evt-secrets-get-001"),
			EventName:   aws.String("GetSecretValue"),
			EventSource: aws.String("secretsmanager.amazonaws.com"),
			EventTime:   aws.Time(tR),
			Username:    aws.String("ci-service-account"),
			ReadOnly:    aws.String("true"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::SecretsManager::Secret"), ResourceName: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","userName":"ci-service-account"},"eventSource":"secretsmanager.amazonaws.com","eventName":"GetSecretValue","requestParameters":{"secretId":"prod/database/primary"}}`),
		},
		// EKS event
		{
			EventId:     aws.String("evt-eks-describe-001"),
			EventName:   aws.String("DescribeCluster"),
			EventSource: aws.String("eks.amazonaws.com"),
			EventTime:   aws.Time(tS),
			Username:    aws.String("alice.johnson"),
			ReadOnly:    aws.String("true"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::EKS::Cluster"), ResourceName: aws.String("arn:aws:eks:us-east-1:123456789012:cluster/acme-prod")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","accountId":"123456789012","userName":"alice.johnson"},"eventSource":"eks.amazonaws.com","eventName":"DescribeCluster","requestParameters":{"name":"acme-prod"}}`),
		},
		// CloudFormation event
		{
			EventId:     aws.String("evt-cfn-update-001"),
			EventName:   aws.String("UpdateStack"),
			EventSource: aws.String("cloudformation.amazonaws.com"),
			EventTime:   aws.Time(tT),
			Username:    aws.String("ci-service-account"),
			ReadOnly:    aws.String("false"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceType: aws.String("AWS::CloudFormation::Stack"), ResourceName: aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111")},
			},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","userName":"ci-service-account"},"eventSource":"cloudformation.amazonaws.com","eventName":"UpdateStack","requestParameters":{"stackName":"acme-vpc-stack"}}`),
		},
	}
}
