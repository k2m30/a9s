// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kinesis_related.go contains Kinesis Data Stream related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkKinesisAlarms checks the cache for CloudWatch alarms with StreamName dimension matching this stream.
func checkKinesisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "kinesis", res)
}

// checkKinesisLambda calls lambda:ListEventSourceMappings with the
// EventSourceArn filter set to this stream's ARN (one call per open stream —
// the per-open call budget in docs/related-resources.md) and maps the returned
// FunctionArn entries against the lambda cache. Secondary event sources on a
// given mapping are not relevant here — every mapping returned by the
// EventSourceArn-filtered call already belongs to this stream.
func checkKinesisLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return foundNone("lambda", "streamARN")
	}
	return lambdaEventSourceMappingLambdaCheck(ctx, clients, streamARN, cache)
}

// checkKinesisCFN calls kinesis:ListTagsForStream and looks up the
// aws:cloudformation:stack-name tag in the cfn cache.
func checkKinesisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return foundNone("cfn", "streamName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return NotRead("cfn")
	}
	tagAPI, ok := c.Kinesis.(KinesisListTagsForStreamAPI)
	if !ok {
		return NotRead("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kinesis.ListTagsForStreamOutput, error) {
		return tagAPI.ListTagsForStream(ctx, &kinesis.ListTagsForStreamInput{StreamName: aws.String(streamName)})
	})
	if err != nil {
		return ReadFailed("cfn", err)
	}
	stackName := ""
	for _, tag := range out.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
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
// configured for KMS-at-rest encryption.
func checkKinesisKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return foundNone("kms", "streamName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return NotRead("kms")
	}
	descAPI, ok := c.Kinesis.(KinesisDescribeStreamSummaryAPI)
	if !ok {
		return NotRead("kms")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kinesis.DescribeStreamSummaryOutput, error) {
		return descAPI.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(streamName)})
	})
	if err != nil {
		return ReadFailed("kms", err)
	}
	if out.StreamDescriptionSummary == nil || out.StreamDescriptionSummary.KeyId == nil || *out.StreamDescriptionSummary.KeyId == "" {
		return foundNone("kms", "out.StreamDescriptionSummary.KeyId")
	}
	keyID := kmsRefFromField(*out.StreamDescriptionSummary.KeyId, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkKinesisDDB is a reverse-scan checker for the kinesis→ddb relationship.
// It iterates cache["ddb"]; for each DynamoDB table it calls
// dynamodb:DescribeKinesisStreamingDestination and checks if any destination's
// StreamArn matches this Kinesis stream's ARN.
func checkKinesisDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return foundNone("ddb", "streamARN")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.DynamoDB == nil {
		return NotRead("ddb")
	}
	api, ok := c.DynamoDB.(DynamoDBDescribeKinesisStreamingDestinationAPI)
	if !ok {
		return NotRead("ddb")
	}

	ddbList, truncated, loaded := cachedRelatedList(cache, "ddb")
	if !loaded {
		return NotRead("ddb")
	}

	var ids []string
	var reads rowReads
	for _, ddbRes := range ddbList {
		tableName := ddbRes.ID
		if tableName == "" {
			reads.missed()
			continue
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
			return api.DescribeKinesisStreamingDestination(ctx, &dynamodb.DescribeKinesisStreamingDestinationInput{
				TableName: aws.String(tableName),
			})
		})
		if err != nil {
			reads.fail(tableName, err)
			continue
		}
		reads.read++
		for _, dest := range out.KinesisDataStreamDestinations {
			if dest.StreamArn != nil && *dest.StreamArn == streamARN {
				ids = append(ids, tableName)
				break
			}
		}
	}
	return reads.answer("ddb", "kinesis-related: DescribeKinesisStreamingDestination", ids, truncated)
}
