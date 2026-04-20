// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// DynamoDBFake implements aws.DynamoDBAPI against fixture data loaded at construction time.
type DynamoDBFake struct {
	fix *fixtures.DynamoDBFixtures
}

// NewDynamoDB constructs a DynamoDBFake backed by fixture data from the fixtures package.
func NewDynamoDB() *DynamoDBFake {
	return &DynamoDBFake{fix: fixtures.NewDynamoDBFixtures()}
}

func (f *DynamoDBFake) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{TableNames: f.fix.TableNames}, nil
}

func (f *DynamoDBFake) DescribeTable(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	name := aws.ToString(input.TableName)
	tbl, ok := f.fix.Tables[name]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: table %q not found", name)
	}
	return &dynamodb.DescribeTableOutput{Table: tbl}, nil
}

func (f *DynamoDBFake) DescribeContinuousBackups(_ context.Context, input *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	name := aws.ToString(input.TableName)
	if _, ok := f.fix.Tables[name]; !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: table %q not found", name)
	}
	return &dynamodb.DescribeContinuousBackupsOutput{}, nil
}

// DescribeKinesisStreamingDestination returns Kinesis streaming destinations for the given table.
func (f *DynamoDBFake) DescribeKinesisStreamingDestination(_ context.Context, input *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	name := aws.ToString(input.TableName)
	dests, ok := f.fix.KinesisDestinations[name]
	if !ok {
		return &dynamodb.DescribeKinesisStreamingDestinationOutput{TableName: input.TableName}, nil
	}
	return &dynamodb.DescribeKinesisStreamingDestinationOutput{
		TableName:                     input.TableName,
		KinesisDataStreamDestinations: dests,
	}, nil
}
