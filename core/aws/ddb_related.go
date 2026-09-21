// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDdbKMS reads SSEDescription.KMSMasterKeyArn from the TableDescription RawStruct.
func checkDdbKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if table.SSEDescription == nil || table.SSEDescription.KMSMasterKeyArn == nil {
		return resource.ProvenZero("kms", "table.SSEDescription.KMSMasterKeyArn")
	}
	return kmsRelated(ctx, clients, cache, []string{*table.SSEDescription.KMSMasterKeyArn})
}

func checkDdbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "ddb", res)
}

// checkDdbBackup resolves AWS Backup plans that cover this DynamoDB table by
// reverse-scanning the already-loaded backup list cache through
// BackupPlanCovers. The row carries no tags, so a plan whose verdict turns on
// a tag clause leaves the answer undecided. No live API call is made.
func checkDdbBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tableARN := res.Fields["arn"]
	if tableARN == "" {
		return resource.ProvenZero("backup", "tableARN")
	}
	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return resource.ErrorRelated("backup", err)
	}
	if backupList == nil {
		return resource.UnknownRelated("backup")
	}
	return backupPivot(backupList, truncated, backupTarget{arn: tableARN, unread: "ListTagsOfResource"})
}

// checkDdbKinesis resolves Kinesis Data Streams connected to this DynamoDB table
// via dynamodb:DescribeKinesisStreamingDestination (1 API call).
// KinesisDataStreamDestinations[].StreamArn values are returned as resource IDs.
func checkDdbKinesis(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tableName := res.ID
	if tableName == "" {
		return resource.ProvenZero("kinesis", "tableName")
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
	var arns []string
	for _, dest := range out.KinesisDataStreamDestinations {
		arns = append(arns, aws.ToString(dest.StreamArn))
	}
	return relatedRefs("kinesis", arns, refContext(clients, cache, "kinesis"))
}

// checkDdbLambda finds Lambda functions wired to this DynamoDB table's stream
// (live API). DDB Streams are consumed through
// lambda:ListEventSourceMappings. A stream ARN carries the moment the stream
// was created, and disabling and re-enabling streams on a table mints a new
// one while a mapping built against the old stream keeps the old ARN, so the
// mappings are matched on the table each source ARN names rather than on the
// table's current stream. Lambda FunctionConfiguration does not embed
// event-source info, so there is no cache-only path. Returns an unknown
// result when no live clients are available.
func checkDdbLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	if table.LatestStreamArn == nil || *table.LatestStreamArn == "" {
		// Streams not enabled on this table — no Lambda triggers are possible.
		return resource.ProvenZero("lambda", "table.LatestStreamArn")
	}
	rc := refContext(clients, cache, "ddb")
	return lambdaEventSourceMappingsNaming(ctx, clients, cache, lambda.ListEventSourceMappingsInput{}, func(sourceARN string) bool {
		id, ok := resource.ResolveRef("ddb", sourceARN, rc)
		return ok && id == res.ID
	})
}
