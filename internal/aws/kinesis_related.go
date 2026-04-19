// kinesis_related.go contains Kinesis Data Stream related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("kinesis", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkKinesisAlarms, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkKinesisLambda, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkKinesisCFN},
		{TargetType: "ddb", DisplayName: "DynamoDB Streams", Checker: checkKinesisDDB, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkKinesisKMS},
	})

	// kinesisstypes.StreamSummary (list response): no navigable fields — KeyId/EncryptionType
	// are on DescribeStream's StreamDescriptionSummary, not the list summary used as RawStruct.
}

// checkKinesisAlarms checks the cache for CloudWatch alarms with StreamName dimension matching this stream.
func checkKinesisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := kinesisRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkKinesisLambda scans the Lambda cache for functions whose first
// EventSourceArn (captured in Fields["event_source_arn"] at fetch time) matches
// this stream's ARN. Pattern C — uses the lambda cache enriched via
// FetchLambdaFunctionsPageWithEventSources. Secondary event sources are not
// captured in the field (only the first is stored), so this check may
// under-count; that's a known cache limitation, not a stub.
func checkKinesisLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	lambdaList, truncated, err := kinesisRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, fn := range lambdaList {
		if fn.Fields["event_source_arn"] == streamARN {
			ids = append(ids, fn.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("lambda")
	}
	return relatedResult("lambda", ids)
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
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	tagAPI, ok := c.Kinesis.(KinesisListTagsForStreamAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := tagAPI.ListTagsForStream(ctx, &kinesis.ListTagsForStreamInput{StreamName: aws.String(streamName)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
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
	cfnList, truncated, err := kinesisRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
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
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	descAPI, ok := c.Kinesis.(KinesisDescribeStreamSummaryAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	out, err := descAPI.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(streamName)})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if out.StreamDescriptionSummary == nil || out.StreamDescriptionSummary.KeyId == nil || *out.StreamDescriptionSummary.KeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *out.StreamDescriptionSummary.KeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// kinesisRelatedResources returns the resource list for target from cache or by fetching the first page.
func kinesisRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
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
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1}
	}
	api, ok := c.DynamoDB.(DynamoDBDescribeKinesisStreamingDestinationAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1}
	}

	entry, ok := cache["ddb"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ddb"}
	}

	var ids []string
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
	result.Approximate = entry.IsTruncated
	return result
}
