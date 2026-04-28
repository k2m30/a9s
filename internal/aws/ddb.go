package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ddb", []string{"table_name", "status", "item_count", "size_bytes", "billing_mode"})

	resource.RegisterPaginated("ddb", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDynamoDBTablesPage(ctx, c.DynamoDB, c.DynamoDB, continuationToken)
	})

	resource.RegisterRelated("ddb", []resource.RelatedDef{
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDdbKMS},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDdbAlarm, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkDdbLambda},
		{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkDdbKinesis},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDdbBackup},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDdbLogs, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkDdbVPCE, NeedsTargetCache: true},
	})

	// ddbtypes.TableDescription: SSEDescription.KMSMasterKeyArn
	resource.RegisterDefaultNavFields("ddb", []resource.NavigableField{
		{FieldPath: "SSEDescription.KMSMasterKeyArn", TargetType: "kms"},
	})
}

// computeDDBFindings returns the ordered Wave-1 findings for a DynamoDB table.
// Returns nil for ACTIVE (healthy) tables.
func computeDDBFindings(ts ddbtypes.TableStatus) []domain.Finding {
	switch ts {
	case ddbtypes.TableStatusActive:
		return nil
	case ddbtypes.TableStatusCreating:
		return []domain.Finding{{Code: CodeDDBCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusUpdating:
		return []domain.Finding{{Code: CodeDDBUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusDeleting:
		return []domain.Finding{{Code: CodeDDBDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusArchiving:
		return []domain.Finding{{Code: CodeDDBArchiving, Phrase: "archiving", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusInaccessibleEncryptionCredentials:
		return []domain.Finding{{Code: CodeDDBKMSKeyInaccessible, Phrase: "kms key inaccessible", Severity: domain.SevBroken, Source: "wave1"}}
	case ddbtypes.TableStatusArchived:
		return []domain.Finding{{Code: CodeDDBArchivedKMSLost, Phrase: "archived: kms key lost", Severity: domain.SevBroken, Source: "wave1"}}
	default:
		return nil
	}
}

// FetchDynamoDBTables calls the DynamoDB ListTables/DescribeTable APIs and
// returns all pages of tables. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchDynamoDBTables(ctx context.Context, listAPI DDBListTablesAPI, describeAPI DDBDescribeTableAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchDynamoDBTablesPage(ctx, listAPI, describeAPI, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchDynamoDBTablesPage performs a two-step fetch: ListTables (single page) to get
// names, then DescribeTable per table for full details.
// Pass an empty continuationToken for the first page.
func FetchDynamoDBTablesPage(ctx context.Context, listAPI DDBListTablesAPI, describeAPI DDBDescribeTableAPI, continuationToken string) (resource.FetchResult, error) {
	input := &dynamodb.ListTablesInput{
		Limit: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.ExclusiveStartTableName = &continuationToken
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.ListTablesOutput, error) {
		return listAPI.ListTables(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing DynamoDB tables: %w", err)
	}

	var failures []string
	var resources []resource.Resource
	for _, tableName := range listOutput.TableNames {
		descOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*dynamodb.DescribeTableOutput, error) {
			return describeAPI.DescribeTable(ctx, &dynamodb.DescribeTableInput{
				TableName: aws.String(tableName),
			})
		})
		if err != nil {
			// Surface per-table failures to the error log so operators see
			// permission/throttle issues instead of a silently short list.
			failures = append(failures, fmt.Sprintf("%s: %v", tableName, err))
			continue
		}

		table := descOutput.Table
		if table == nil {
			continue
		}

		name := ""
		if table.TableName != nil {
			name = *table.TableName
		}

		findings := computeDDBFindings(table.TableStatus)
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
		}

		itemCount := ""
		if table.ItemCount != nil {
			itemCount = fmt.Sprintf("%d", *table.ItemCount)
		}

		sizeBytes := ""
		if table.TableSizeBytes != nil {
			sizeBytes = formatBytes(*table.TableSizeBytes)
		}

		billingMode := ""
		if table.BillingModeSummary != nil {
			billingMode = string(table.BillingModeSummary.BillingMode)
		}

		arn := ""
		if table.TableArn != nil {
			arn = *table.TableArn
		}

		r := resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"table_name":   name,
				"status":       statusPhrase,
				"item_count":   itemCount,
				"size_bytes":   sizeBytes,
				"billing_mode": billingMode,
				"arn":          arn,
			},
			Findings:  findings,
			RawStruct: table,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if listOutput.LastEvaluatedTableName != nil {
		nextToken = *listOutput.LastEvaluatedTableName
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, AggregateFailures("ddb: DescribeTable", failures, len(listOutput.TableNames))
}
