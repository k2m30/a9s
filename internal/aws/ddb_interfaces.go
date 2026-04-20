package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DDBListTablesAPI defines the interface for the DynamoDB ListTables operation.
type DDBListTablesAPI interface {
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
}

// DDBDescribeTableAPI defines the interface for the DynamoDB DescribeTable operation.
type DDBDescribeTableAPI interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// DynamoDBDescribeContinuousBackupsAPI defines the interface for the DynamoDB DescribeContinuousBackups operation.
type DynamoDBDescribeContinuousBackupsAPI interface {
	DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error)
}

// DynamoDBDescribeKinesisStreamingDestinationAPI defines the interface for the DynamoDB DescribeKinesisStreamingDestination operation.
type DynamoDBDescribeKinesisStreamingDestinationAPI interface {
	DescribeKinesisStreamingDestination(ctx context.Context, params *dynamodb.DescribeKinesisStreamingDestinationInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error)
}

// DynamoDBAPI is the aggregate interface covering all DynamoDB operations used by a9s fetchers.
// *dynamodb.Client structurally satisfies this interface.
type DynamoDBAPI interface {
	DDBListTablesAPI
	DDBDescribeTableAPI
	DynamoDBDescribeContinuousBackupsAPI
	DynamoDBDescribeKinesisStreamingDestinationAPI
}
