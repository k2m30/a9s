// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// DynamoDBFake implements aws.DynamoDBAPI against fixture data loaded at construction time.
type DynamoDBFake struct {
	fix *fixtures.DDBFixtures
}

// NewDynamoDB constructs a DynamoDBFake backed by fixture data from the fixtures package.
func NewDynamoDB() *DynamoDBFake {
	return &DynamoDBFake{fix: fixtures.NewDDBFixtures()}
}

func (f *DynamoDBFake) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	names := make([]string, 0, len(f.fix.Tables))
	for _, t := range f.fix.Tables {
		names = append(names, aws.ToString(t.TableName))
	}
	return &dynamodb.ListTablesOutput{TableNames: names}, nil
}

func (f *DynamoDBFake) DescribeTable(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	name := aws.ToString(input.TableName)
	for _, t := range f.fix.Tables {
		if aws.ToString(t.TableName) == name {
			return &dynamodb.DescribeTableOutput{Table: t}, nil
		}
	}
	return nil, fmt.Errorf("ResourceNotFoundException: table %q not found", name)
}

// DescribeContinuousBackups returns PITR status per table.
// Tables in the ContinuousBackups map get their exact status; all others
// default to PITR ENABLED so that Healthy tables produce no findings.
func (f *DynamoDBFake) DescribeContinuousBackups(_ context.Context, input *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	name := aws.ToString(input.TableName)
	if desc, ok := f.fix.ContinuousBackups[name]; ok {
		return &dynamodb.DescribeContinuousBackupsOutput{ContinuousBackupsDescription: desc}, nil
	}
	// Default: PITR ENABLED — no finding emitted.
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			ContinuousBackupsStatus: ddbtypes.ContinuousBackupsStatusEnabled,
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
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
