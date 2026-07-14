// kinesis_related.go contains Kinesis Data Stream related-resource checker functions.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkKinesisAlarms checks the cache for CloudWatch alarms with StreamName dimension matching this stream.
func checkKinesisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "StreamName" && d.Value != nil && *d.Value == streamName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}

// checkKinesisLambda calls lambda:ListEventSourceMappings with the
// EventSourceArn filter set to this stream's ARN (one call per open stream —
// budget rule 7 in docs/related-resources.md) and maps the returned
// FunctionArn entries against the lambda cache. Secondary event sources on a
// given mapping are not relevant here — every mapping returned by the
// EventSourceArn-filtered call already belongs to this stream.
func checkKinesisLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	return lambdaEventSourceMappingLambdaCheck(ctx, clients, streamARN, cache)
}

// checkKinesisCFN calls kinesis:ListTagsForStream and looks up the
// aws:cloudformation:stack-name tag in the cfn cache. Pattern C.
func checkKinesisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return resource.UnknownRelated("cfn")
	}
	tagAPI, ok := c.Kinesis.(KinesisListTagsForStreamAPI)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kinesis.ListTagsForStreamOutput, error) {
		return tagAPI.ListTagsForStream(ctx, &kinesis.ListTagsForStreamInput{StreamName: aws.String(streamName)})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	stackName := ""
	for _, tag := range out.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkKinesisKMS calls kinesis:DescribeStreamSummary and returns the KeyId
// configured for KMS-at-rest encryption. Pattern C.
func checkKinesisKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return resource.UnknownRelated("kms")
	}
	descAPI, ok := c.Kinesis.(KinesisDescribeStreamSummaryAPI)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kinesis.DescribeStreamSummaryOutput, error) {
		return descAPI.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(streamName)})
	})
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if out.StreamDescriptionSummary == nil || out.StreamDescriptionSummary.KeyId == nil || *out.StreamDescriptionSummary.KeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := kmsKeyIDFromField(*out.StreamDescriptionSummary.KeyId, res.Type)
	return relatedResult("kms", []string{keyID})
}

// checkKinesisDDB is a reverse-scan checker for the kinesis→ddb relationship.
// Pattern C+reverse: iterate cache["ddb"]; for each DynamoDB table call
// dynamodb:DescribeKinesisStreamingDestination and check if any destination's
// StreamArn matches this Kinesis stream's ARN.
// NeedsTargetCache: true.
func checkKinesisDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.DynamoDB == nil {
		return resource.UnknownRelated("ddb")
	}
	api, ok := c.DynamoDB.(DynamoDBDescribeKinesisStreamingDestinationAPI)
	if !ok {
		return resource.UnknownRelated("ddb")
	}

	entry, ok := cache["ddb"]
	if !ok {
		return resource.UnknownRelated("ddb")
	}

	var ids []string
	var failures []string
	total := len(entry.Resources)
	for _, ddbRes := range entry.Resources {
		tableName := ddbRes.ID
		if tableName == "" {
			continue
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
			return api.DescribeKinesisStreamingDestination(ctx, &dynamodb.DescribeKinesisStreamingDestinationInput{
				TableName: aws.String(tableName),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", tableName, err))
			continue
		}
		for _, dest := range out.KinesisDataStreamDestinations {
			if dest.StreamArn != nil && *dest.StreamArn == streamARN {
				ids = append(ids, tableName)
				break
			}
		}
	}
	result := relatedResult("ddb", ids)
	result.Truncated = entry.IsTruncated
	result.Err = AggregateFailures("kinesis-related: DescribeKinesisStreamingDestination", failures, total)
	return result
}
