// Package fixtures provides DynamoDB fixture data for the DynamoDB fake.
package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DDBFixtures holds typed fixture data for DynamoDB tables.
type DDBFixtures struct {
	// Tables is the ordered list of TableDescription objects.
	// The fetcher calls ListTables then DescribeTable per entry.
	Tables []*ddbtypes.TableDescription
	// ContinuousBackups maps table name → ContinuousBackupsDescription for
	// the DescribeContinuousBackups fake. Tables absent from this map are
	// treated as PITR ENABLED (healthy default).
	ContinuousBackups map[string]*ddbtypes.ContinuousBackupsDescription
	// KinesisDestinations maps table name → []KinesisDataStreamDestination.
	KinesisDestinations map[string][]ddbtypes.KinesisDataStreamDestination
}

// Stable IDs and ARNs — imported by sibling fixture files and tests.
const (
	// orders-prod — graph-root, Healthy, all pivots wired.
	OrdersProdID  = "orders-prod"
	OrdersProdARN = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod"

	// orders-prod stream ARN — used by lambda ESM and kinesis destination wiring.
	OrdersProdStreamARN = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod/stream/2026-01-01T00:00:00.000"

	// orders-prod KMS key — referenced by kms.go fixture.
	OrdersProdKMSKeyID  = "orders-prod-cmk-0001"
	OrdersProdKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/orders-prod-cmk-0001"

	// orders-prod Kinesis CDC stream — referenced by kinesis.go fixture.
	OrdersProdKinesisStream    = "orders-prod-cdc"
	OrdersProdKinesisStreamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/orders-prod-cdc"

	// orders-prod Lambda projector — referenced by lambda.go fixture.
	OrdersProdLambdaName = "orders-projector"
	OrdersProdLambdaARN  = "arn:aws:lambda:us-east-1:123456789012:function:orders-projector"

	// sessions-creating — CREATING transitional state.
	SessionsCreatingID  = "sessions-creating"
	SessionsCreatingARN = "arn:aws:dynamodb:us-east-1:123456789012:table/sessions-creating"

	// sessions-updating — UPDATING transitional state.
	SessionsUpdatingID  = "sessions-updating"
	SessionsUpdatingARN = "arn:aws:dynamodb:us-east-1:123456789012:table/sessions-updating"

	// analytics-deleting — DELETING transitional state.
	AnalyticsDeletingID  = "analytics-deleting"
	AnalyticsDeletingARN = "arn:aws:dynamodb:us-east-1:123456789012:table/analytics-deleting"

	// legacy-archiving — ARCHIVING transitional state.
	LegacyArchivingID  = "legacy-archiving"
	LegacyArchivingARN = "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-archiving"

	// legacy-kms-lost — INACCESSIBLE_ENCRYPTION_CREDENTIALS broken state.
	LegacyKMSLostID  = "legacy-kms-lost"
	LegacyKMSLostARN = "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-kms-lost"

	// legacy-archived — ARCHIVED broken state + PITR DISABLED (multi-W2 stack).
	LegacyArchivedID  = "legacy-archived"
	LegacyArchivedARN = "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-archived"

	// audit-pitr-off — ACTIVE + PITR DISABLED (~-severity finding only).
	AuditPITROffID  = "audit-pitr-off"
	AuditPITROffARN = "arn:aws:dynamodb:us-east-1:123456789012:table/audit-pitr-off"

)

// NewDDBFixtures returns a fully-populated DDBFixtures for demo and tests.
var sharedDDBFixtures = sync.OnceValue(func() *DDBFixtures {
	return &DDBFixtures{
		Tables:              buildDDBTables(),
		ContinuousBackups:   buildDDBContinuousBackups(),
		KinesisDestinations: buildDDBKinesisDestinations(),
	}
})

func NewDDBFixtures() *DDBFixtures {
	return sharedDDBFixtures()
}

// pitrEnabled returns a ContinuousBackupsDescription with PITR enabled.
func pitrEnabled() *ddbtypes.ContinuousBackupsDescription {
	return &ddbtypes.ContinuousBackupsDescription{
		ContinuousBackupsStatus: ddbtypes.ContinuousBackupsStatusEnabled,
		PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
			PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			EarliestRestorableDateTime: aws.Time(time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)),
			LatestRestorableDateTime:   aws.Time(time.Now().UTC()),
		},
	}
}

// pitrDisabled returns a ContinuousBackupsDescription with PITR disabled.
func pitrDisabled() *ddbtypes.ContinuousBackupsDescription {
	return &ddbtypes.ContinuousBackupsDescription{
		ContinuousBackupsStatus: ddbtypes.ContinuousBackupsStatusEnabled,
		PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
			PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusDisabled,
		},
	}
}

// buildDDBContinuousBackups returns PITR state per table name.
// Tables not in this map are PITR ENABLED (healthy default — not emitted as findings).
func buildDDBContinuousBackups() map[string]*ddbtypes.ContinuousBackupsDescription {
	return map[string]*ddbtypes.ContinuousBackupsDescription{
		OrdersProdID:      pitrEnabled(),
		SessionsCreatingID: pitrEnabled(),
		SessionsUpdatingID: pitrEnabled(),
		AnalyticsDeletingID: pitrEnabled(),
		LegacyArchivingID:  pitrEnabled(),
		LegacyKMSLostID:    pitrEnabled(),
		// legacy-archived: PITR DISABLED to exercise multi-W2 stacking (+1 suffix).
		LegacyArchivedID: pitrDisabled(),
		// audit-pitr-off: PITR DISABLED to exercise the ~ glyph on a Healthy row.
		AuditPITROffID: pitrDisabled(),
	}
}

// buildDDBKinesisDestinations returns Kinesis streaming destinations per table name.
// orders-prod streams to orders-prod-cdc for real-time change-data-capture.
func buildDDBKinesisDestinations() map[string][]ddbtypes.KinesisDataStreamDestination {
	return map[string][]ddbtypes.KinesisDataStreamDestination{
		OrdersProdID: {
			{
				StreamArn:         aws.String(OrdersProdKinesisStreamARN),
				DestinationStatus: ddbtypes.DestinationStatusActive,
				DestinationStatusDescription: aws.String("Stream is active"),
			},
		},
	}
}

func buildDDBTables() []*ddbtypes.TableDescription {
	return []*ddbtypes.TableDescription{
		// -----------------------------------------------------------------------
		// orders-prod — graph-root, Healthy, all pivots wired.
		// -----------------------------------------------------------------------
		{
			TableName:   aws.String(OrdersProdID),
			TableArn:    aws.String(OrdersProdARN),
			TableId:     aws.String("d0b1c2d3-0001-0001-0001-000000000001"),
			TableStatus: ddbtypes.TableStatusActive,
			ItemCount:   aws.Int64(12_345_678),
			TableSizeBytes: aws.Int64(4_294_967_296), // 4 GiB
			CreationDateTime: aws.Time(time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			DeletionProtectionEnabled: aws.Bool(true),
			SSEDescription: &ddbtypes.SSEDescription{
				Status:          ddbtypes.SSEStatusEnabled,
				SSEType:         ddbtypes.SSETypeKms,
				KMSMasterKeyArn: aws.String(OrdersProdKMSKeyARN),
			},
			StreamSpecification: &ddbtypes.StreamSpecification{
				StreamEnabled:  aws.Bool(true),
				StreamViewType: ddbtypes.StreamViewTypeNewAndOldImages,
			},
			LatestStreamArn:   aws.String(OrdersProdStreamARN),
			LatestStreamLabel: aws.String("2026-01-01T00:00:00.000"),
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
					IndexArn:       aws.String(OrdersProdARN + "/index/CustomerId-index"),
					IndexStatus:    ddbtypes.IndexStatusActive,
					IndexSizeBytes: aws.Int64(536_870_912),
					ItemCount:      aws.Int64(12_345_678),
				},
			},
		},
		// -----------------------------------------------------------------------
		// sessions-creating — CREATING transitional state (Warning).
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(SessionsCreatingID),
			TableArn:         aws.String(SessionsCreatingARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000002"),
			TableStatus:      ddbtypes.TableStatusCreating,
			ItemCount:        aws.Int64(0),
			TableSizeBytes:   aws.Int64(0),
			CreationDateTime: aws.Time(time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("SessionId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("SessionId"), KeyType: ddbtypes.KeyTypeHash},
			},
		},
		// -----------------------------------------------------------------------
		// sessions-updating — UPDATING transitional state (Warning).
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(SessionsUpdatingID),
			TableArn:         aws.String(SessionsUpdatingARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000003"),
			TableStatus:      ddbtypes.TableStatusUpdating,
			ItemCount:        aws.Int64(1_023_456),
			TableSizeBytes:   aws.Int64(524_288_000),
			CreationDateTime: aws.Time(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("SessionId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("SessionId"), KeyType: ddbtypes.KeyTypeHash},
			},
		},
		// -----------------------------------------------------------------------
		// analytics-deleting — DELETING transitional state (Warning).
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(AnalyticsDeletingID),
			TableArn:         aws.String(AnalyticsDeletingARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000004"),
			TableStatus:      ddbtypes.TableStatusDeleting,
			ItemCount:        aws.Int64(5_000_000),
			TableSizeBytes:   aws.Int64(2_147_483_648),
			CreationDateTime: aws.Time(time.Date(2024, 3, 15, 7, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("EventId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("EventId"), KeyType: ddbtypes.KeyTypeHash},
			},
		},
		// -----------------------------------------------------------------------
		// legacy-archiving — ARCHIVING transitional state (Warning).
		// ArchivalSummary is nil (archival not yet finalized).
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(LegacyArchivingID),
			TableArn:         aws.String(LegacyArchivingARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000005"),
			TableStatus:      ddbtypes.TableStatusArchiving,
			ItemCount:        aws.Int64(234_567),
			TableSizeBytes:   aws.Int64(104_857_600),
			CreationDateTime: aws.Time(time.Date(2023, 8, 1, 6, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("Id"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("Id"), KeyType: ddbtypes.KeyTypeHash},
			},
			// ArchivalSummary intentionally nil — archival not yet finalized.
		},
		// -----------------------------------------------------------------------
		// legacy-kms-lost — INACCESSIBLE_ENCRYPTION_CREDENTIALS broken state.
		// SSEDescription points at a now-deleted CMK.
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(LegacyKMSLostID),
			TableArn:         aws.String(LegacyKMSLostARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000006"),
			TableStatus:      ddbtypes.TableStatusInaccessibleEncryptionCredentials,
			ItemCount:        aws.Int64(89_000),
			TableSizeBytes:   aws.Int64(52_428_800),
			CreationDateTime: aws.Time(time.Date(2023, 1, 20, 11, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			SSEDescription: &ddbtypes.SSEDescription{
				Status:          ddbtypes.SSEStatusEnabled,
				SSEType:         ddbtypes.SSETypeKms,
				KMSMasterKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/legacy-prod-cmk-deleted"),
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("Id"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("Id"), KeyType: ddbtypes.KeyTypeHash},
			},
		},
		// -----------------------------------------------------------------------
		// legacy-archived — ARCHIVED broken state + PITR DISABLED.
		// Exercises multi-W2 stacking (+1 suffix) and U7c detail-view-shows-both.
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(LegacyArchivedID),
			TableArn:         aws.String(LegacyArchivedARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000007"),
			TableStatus:      ddbtypes.TableStatusArchived,
			ItemCount:        aws.Int64(0),
			TableSizeBytes:   aws.Int64(0),
			CreationDateTime: aws.Time(time.Date(2022, 5, 1, 9, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModePayPerRequest,
			},
			SSEDescription: &ddbtypes.SSEDescription{
				Status:          ddbtypes.SSEStatusEnabled,
				SSEType:         ddbtypes.SSETypeKms,
				KMSMasterKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/legacy-archived-cmk-lost"),
			},
			ArchivalSummary: &ddbtypes.ArchivalSummary{
				ArchivalReason:    aws.String("INACCESSIBLE_ENCRYPTION_CREDENTIALS"),
				ArchivalDateTime:  aws.Time(time.Date(2024, 11, 2, 3, 15, 0, 0, time.UTC)),
				ArchivalBackupArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-legacy-archived-20241102"),
			},
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("Id"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("Id"), KeyType: ddbtypes.KeyTypeHash},
			},
		},
		// -----------------------------------------------------------------------
		// audit-pitr-off — ACTIVE, PITR DISABLED, PROVISIONED billing.
		// Healthy row with ~ glyph (U3/U11). No streams, no CMK.
		// -----------------------------------------------------------------------
		{
			TableName:        aws.String(AuditPITROffID),
			TableArn:         aws.String(AuditPITROffARN),
			TableId:          aws.String("d0b1c2d3-0001-0001-0001-000000000008"),
			TableStatus:      ddbtypes.TableStatusActive,
			ItemCount:        aws.Int64(1_234_567),
			TableSizeBytes:   aws.Int64(268_435_456),
			CreationDateTime: aws.Time(time.Date(2023, 11, 15, 8, 0, 0, 0, time.UTC)),
			BillingModeSummary: &ddbtypes.BillingModeSummary{
				BillingMode: ddbtypes.BillingModeProvisioned,
			},
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			DeletionProtectionEnabled: aws.Bool(true),
			AttributeDefinitions: []ddbtypes.AttributeDefinition{
				{AttributeName: aws.String("AuditId"), AttributeType: ddbtypes.ScalarAttributeTypeS},
				{AttributeName: aws.String("Timestamp"), AttributeType: ddbtypes.ScalarAttributeTypeN},
			},
			KeySchema: []ddbtypes.KeySchemaElement{
				{AttributeName: aws.String("AuditId"), KeyType: ddbtypes.KeyTypeHash},
				{AttributeName: aws.String("Timestamp"), KeyType: ddbtypes.KeyTypeRange},
			},
		},
	}
}
