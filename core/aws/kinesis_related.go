// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kinesis_related.go contains Kinesis Data Stream related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkKinesisAlarms checks the cache for CloudWatch alarms with StreamName dimension matching this stream.
func checkKinesisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "kinesis", res)
}

// checkKinesisLambda reports the functions reading this stream: the event
// source mappings on the stream's ARN, and those on each of its enhanced
// fan-out consumers, which a mapping names by the consumer's own ARN
// ("stream/<name>/consumer/<consumer>:<timestamp>",
// https://docs.aws.amazon.com/lambda/latest/dg/with-kinesis.html). The
// consumers are kinesis:ListStreamConsumers.
func checkKinesisLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamARN := res.Fields["stream_arn"]
	if streamARN == "" {
		return keyMissing("lambda", "streamARN")
	}
	stream := lambdaEventSourceMappingLambdaCheck(ctx, clients, streamARN, cache)
	if stream.Err() != nil {
		return stream
	}
	consumers, read := kinesisConsumerARNs(ctx, clients, streamARN)
	reads := []relatedRead{read, readOf(stream)}
	for _, arn := range consumers {
		reads = append(reads, readOf(lambdaEventSourceMappingLambdaCheck(ctx, clients, arn, cache)))
	}
	return relatedAnswer("lambda", joinReads(reads...))
}

// kinesisConsumerARNs lists the ARNs of the stream's enhanced fan-out
// consumers, and what the listing read.
func kinesisConsumerARNs(ctx context.Context, clients any, streamARN string) ([]string, relatedRead) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return nil, relatedRead{unread: true}
	}
	api, ok := c.Kinesis.(KinesisListStreamConsumersAPI)
	if !ok {
		return nil, relatedRead{unread: true}
	}
	consumers, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]kinesistypes.Consumer, *string, error) {
		out, err := api.ListStreamConsumers(ctx, &kinesis.ListStreamConsumersInput{StreamARN: aws.String(streamARN), NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Consumers, out.NextToken, nil
	})
	arns := make([]string, 0, len(consumers))
	for _, cons := range consumers {
		arns = append(arns, aws.ToString(cons.ConsumerARN))
	}
	return arns, pagedRead(complete, err)
}

// checkKinesisCFN calls kinesis:ListTagsForStream and looks up the
// aws:cloudformation:stack-name tag in the cfn cache.
func checkKinesisCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return keyMissing("cfn", "streamName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Kinesis == nil {
		return NotRead("cfn")
	}
	tagAPI, ok := c.Kinesis.(KinesisListTagsForStreamAPI)
	if !ok {
		return NotRead("cfn")
	}
	// ListTagsForStream pages by HasMoreTags, resuming after
	// ExclusiveStartTagKey, the last key read
	// (https://docs.aws.amazon.com/kinesis/latest/APIReference/API_ListTagsForStream.html).
	tags, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, lastKey *string) ([]kinesistypes.Tag, *string, error) {
		out, err := tagAPI.ListTagsForStream(ctx, &kinesis.ListTagsForStreamInput{StreamName: aws.String(streamName), ExclusiveStartTagKey: lastKey})
		if err != nil {
			return nil, nil, err
		}
		if !aws.ToBool(out.HasMoreTags) || len(out.Tags) == 0 {
			return out.Tags, nil, nil
		}
		return out.Tags, out.Tags[len(out.Tags)-1].Key, nil
	})
	stackName := ""
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "aws:cloudformation:stack-name" {
			stackName = aws.ToString(tag.Value)
			break
		}
	}
	if stackName == "" {
		if err != nil {
			return relatedAnswer("cfn", unreadBy(err))
		}
		return relatedAnswer("cfn", relatedRead{partial: !complete})
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
		return keyMissing("kms", "streamName")
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
		return keyMissing("ddb", "streamARN")
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
	ddbList, capped := fanOut(ddbList)
	truncated = truncated || capped
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
			if aws.ToString(dest.StreamArn) == streamARN && ddbDestinationLive(dest) {
				ids = append(ids, tableName)
				break
			}
		}
	}
	return reads.answer("ddb", "kinesis-related: DescribeKinesisStreamingDestination", ids, truncated)
}
