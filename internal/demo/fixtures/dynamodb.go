// Package fixtures provides DynamoDB fixture data for the DynamoDB fake.
package fixtures

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBFixtures holds all DynamoDB domain objects served by the fake.
type DynamoDBFixtures struct {
	// TableNames is the ordered list returned by ListTables.
	TableNames []string
	// Tables maps table name → *ddbtypes.TableDescription (pointer, matching production fetcher).
	Tables map[string]*ddbtypes.TableDescription
	// KinesisDestinations maps table name → []KinesisDataStreamDestination.
	KinesisDestinations map[string][]ddbtypes.KinesisDataStreamDestination
}

// NewDynamoDBFixtures builds and returns a fully-populated DynamoDBFixtures struct.
func NewDynamoDBFixtures() *DynamoDBFixtures {
	tables := buildDynamoDBTables()
	names := make([]string, 0, len(tables))
	tableMap := make(map[string]*ddbtypes.TableDescription, len(tables))
	for i := range tables {
		name := aws.ToString(tables[i].TableName)
		names = append(names, name)
		tableMap[name] = tables[i]
	}
	return &DynamoDBFixtures{
		TableNames:          names,
		Tables:              tableMap,
		KinesisDestinations: buildDynamoDBKinesisDestinations(),
	}
}

// buildDynamoDBKinesisDestinations returns Kinesis streaming destination fixtures.
// acme-orders streams to the acme-orders-stream for real-time analytics.
func buildDynamoDBKinesisDestinations() map[string][]ddbtypes.KinesisDataStreamDestination {
	return map[string][]ddbtypes.KinesisDataStreamDestination{
		"acme-orders": {
			{
				StreamArn:                    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/acme-orders-stream"),
				DestinationStatus:            ddbtypes.DestinationStatusActive,
				DestinationStatusDescription: aws.String("Stream is active"),
			},
		},
	}
}

var ddbNamePool = []string{
	"acme-products", "acme-categories", "acme-carts", "acme-wishlist",
	"acme-reviews", "acme-payments", "acme-refunds", "acme-promotions",
	"acme-subscriptions", "acme-notifications", "acme-search-index",
	"acme-feature-flags", "acme-rate-limits", "acme-locks",
	"acme-counters", "acme-events", "acme-analytics", "acme-config",
}

func buildDynamoDBTables() []*ddbtypes.TableDescription {
	named := []*ddbtypes.TableDescription{
		{
			TableName:        aws.String("acme-orders"),
			TableStatus:      ddbtypes.TableStatusActive,
			TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"),
			TableId:          aws.String("a1b2c3d4-0000-1111-2222-333333333333"),
			ItemCount:        aws.Int64(2458103),
			TableSizeBytes:   aws.Int64(1073741824),
			CreationDateTime: aws.Time(mustTime("2025-02-10T09:00:00Z")),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			DeletionProtectionEnabled: aws.Bool(true),
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("OrderId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
				{AttributeName: aws.String("CustomerId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("OrderId"), KeyType: ddbtypes.KeyTypeHash},
				{AttributeName: aws.String("CustomerId"), KeyType: ddbtypes.KeyTypeRange},
			},
			GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndexDescription{
				{
					IndexName:      aws.String("CustomerId-index"),
					IndexArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders/index/CustomerId-index"),
					IndexStatus:    ddbtypes.IndexStatusActive,
					IndexSizeBytes: aws.Int64(256000000),
					ItemCount:      aws.Int64(2458103),
				},
			},
			SSEDescription: &ddbtypes.SSEDescription{
				Status: ddbtypes.SSEStatusEnabled,
			},
		},
		{
			TableName:        aws.String("acme-sessions"),
			TableStatus:      ddbtypes.TableStatusActive,
			TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-sessions"),
			TableId:          aws.String("b2c3d4e5-0000-1111-2222-333333333333"),
			ItemCount:        aws.Int64(145230),
			TableSizeBytes:   aws.Int64(52428800),
			CreationDateTime: aws.Time(mustTime("2025-03-01T10:00:00Z")),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			DeletionProtectionEnabled: aws.Bool(true),
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("SessionId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("SessionId"), KeyType: ddbtypes.KeyTypeHash},
			},
			SSEDescription: &ddbtypes.SSEDescription{
				Status: ddbtypes.SSEStatusEnabled,
			},
		},
		{
			TableName:        aws.String("acme-inventory"),
			TableStatus:      ddbtypes.TableStatusActive,
			TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-inventory"),
			TableId:          aws.String("c3d4e5f6-0000-1111-2222-333333333333"),
			ItemCount:        aws.Int64(89450),
			TableSizeBytes:   aws.Int64(104857600),
			CreationDateTime: aws.Time(mustTime("2025-01-15T08:00:00Z")),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModeProvisioned,
			},
			DeletionProtectionEnabled: aws.Bool(true),
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("SKU"), AttributeType: ddbtypes.ScalarAttributeTypeS},
				{AttributeName: aws.String("WarehouseId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("SKU"), KeyType: ddbtypes.KeyTypeHash},
				{AttributeName: aws.String("WarehouseId"), KeyType: ddbtypes.KeyTypeRange},
			},
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  aws.Int64(100),
				WriteCapacityUnits: aws.Int64(50),
			},
		},
		{
			TableName:        aws.String("acme-audit-log"),
			TableStatus:      ddbtypes.TableStatusActive,
			TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-audit-log"),
			TableId:          aws.String("d4e5f6a7-0000-1111-2222-333333333333"),
			ItemCount:        aws.Int64(9823741),
			TableSizeBytes:   aws.Int64(5368709120),
			CreationDateTime: aws.Time(mustTime("2024-06-01T07:00:00Z")),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			DeletionProtectionEnabled: aws.Bool(true),
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("EventId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
				{AttributeName: aws.String("Timestamp"), AttributeType: ddbtypes.ScalarAttributeTypeN},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("EventId"), KeyType: ddbtypes.KeyTypeHash},
				{AttributeName: aws.String("Timestamp"), KeyType: ddbtypes.KeyTypeRange},
			},
			SSEDescription: &ddbtypes.SSEDescription{
				Status: ddbtypes.SSEStatusEnabled,
			},
		},
	}

	// Generate 18 more tables to reach 22 total.
	for i := range 18 {
		name := ddbNamePool[i]
		named = append(named, &ddbtypes.TableDescription{
			TableName:        aws.String(name),
			TableStatus:      ddbtypes.TableStatusActive,
			TableArn:         aws.String(fmt.Sprintf("arn:aws:dynamodb:us-east-1:123456789012:table/%s", name)),
			TableId:          aws.String(fmt.Sprintf("e5f6a7b8-00%02d-1111-2222-333333333333", i)),
			ItemCount:        aws.Int64(int64(1000 + i*500)),
			TableSizeBytes:   aws.Int64(int64(1048576 * (i + 1))),
			CreationDateTime: aws.Time(mustTime(fmt.Sprintf("2025-%02d-%02dT09:00:00Z", 1+(i%12), 1+(i%28)))),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("Id"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("Id"), KeyType: ddbtypes.KeyTypeHash},
			},
		})
	}

	return named
}
