// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDdbKMS reads SSEDescription.KMSMasterKeyArn from the TableDescription RawStruct.
// Pattern F — no cache needed.
func checkDdbKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if table.SSEDescription == nil || table.SSEDescription.KMSMasterKeyArn == nil {
		return resource.KnownRelated("kms", nil, false)
	}
	arn := *table.SSEDescription.KMSMasterKeyArn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := arn[idx+1:]
	return relatedResult("kms", []string{keyID})
}

// checkDdbAlarm searches the alarm cache for alarms with a "TableName" dimension
// matching this DynamoDB table's name.
// Pattern D — dimension-based lookup.
func checkDdbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tableName := res.ID
	if tableName == "" {
		return resource.KnownRelated("alarm", nil, false)
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
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "TableName" && d.Value != nil && *d.Value == tableName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("alarm", nil, true)
	}
	if truncated {
		return truncatedResultDDB("alarm", ids)
	}
	return relatedResult("alarm", ids)
}

// checkDdbBackup resolves AWS Backup plans that cover this DynamoDB table by
// reverse-scanning the already-loaded backup list cache. For each cached plan,
// Fields["resources"] contains a comma-separated list of resource ARNs or
// wildcard patterns (e.g. arn:aws:dynamodb:*:*:table/*); Fields["not_resources"]
// contains exclusion patterns with the same wildcard semantics. A plan covers
// this table iff any Resources entry matches the table's ARN AND no NotResources
// entry matches. No live API call is made — this is a pure cache scan.
func checkDdbBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tableARN := res.Fields["arn"]
	if tableARN == "" {
		return resource.KnownRelated("backup", nil, false)
	}
	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return resource.ErrorRelated("backup", err)
	}
	if backupList == nil {
		return resource.UnknownRelated("backup")
	}
	var ids []string
	for _, planRes := range backupList {
		if BackupPlanCoversARN(planRes.Fields["resources"], planRes.Fields["not_resources"], tableARN) {
			ids = append(ids, planRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("backup", nil, true)
	}
	if truncated {
		return truncatedResultDDB("backup", ids)
	}
	return relatedResult("backup", ids)
}

// checkDdbKinesis resolves Kinesis Data Streams connected to this DynamoDB table
// via dynamodb:DescribeKinesisStreamingDestination (Pattern C: 1 API call).
// KinesisDataStreamDestinations[].StreamArn values are returned as resource IDs.
func checkDdbKinesis(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tableName := res.ID
	if tableName == "" {
		return resource.KnownRelated("kinesis", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.DynamoDB == nil {
		return resource.UnknownRelated("kinesis")
	}
	api, ok := c.DynamoDB.(DynamoDBDescribeKinesisStreamingDestinationAPI)
	if !ok {
		return resource.UnknownRelated("kinesis")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
		return api.DescribeKinesisStreamingDestination(ctx, &dynamodb.DescribeKinesisStreamingDestinationInput{TableName: &tableName})
	})
	if err != nil {
		return resource.ErrorRelated("kinesis", err)
	}
	var ids []string
	for _, dest := range out.KinesisDataStreamDestinations {
		if dest.StreamArn != nil && *dest.StreamArn != "" {
			// Extract stream name from ARN (last ":" segment).
			parts := strings.Split(*dest.StreamArn, "/")
			name := parts[len(parts)-1]
			if name != "" {
				ids = append(ids, name)
			}
		}
	}
	return relatedResult("kinesis", ids)
}

// truncatedResultDDB returns a RelatedCheckResult with Truncated=true when the
// target cache is truncated and matches were found. Later pages may contain
// additional matches, so the displayed count is a lower bound — rendered as "(N+)".
func truncatedResultDDB(target string, ids []string) resource.RelatedCheckResult {
	return resource.KnownRelated(target, ids, true)
}

// checkDdbLambda finds Lambda functions wired to this DynamoDB table's stream
// (Pattern A — live API). DDB Streams are consumed through
// lambda:ListEventSourceMappings; the EventSourceArn on each mapping matches
// the table's LatestStreamArn. Lambda FunctionConfiguration does not embed
// event-source info, so there is no cache-only path. Returns an unknown
// result when no live clients are available.
func checkDdbLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	if table.LatestStreamArn == nil || *table.LatestStreamArn == "" {
		// Streams not enabled on this table — no Lambda triggers are possible.
		return resource.KnownRelated("lambda", nil, false)
	}
	streamARN := *table.LatestStreamArn
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("lambda")
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		EventSourceArn: &streamARN,
	})
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	var ids []string
	for _, m := range out.EventSourceMappings {
		if m.FunctionArn == nil {
			continue
		}
		parts := strings.Split(*m.FunctionArn, ":")
		name := parts[len(parts)-1]
		if name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("lambda", ids)
}
