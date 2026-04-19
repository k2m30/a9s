package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

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
	resource.RegisterNavigableFields("ddb", []resource.NavigableField{
		{FieldPath: "SSEDescription.KMSMasterKeyArn", TargetType: "kms"},
	})
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

	listOutput, err := listAPI.ListTables(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing DynamoDB tables: %w", err)
	}

	var resources []resource.Resource
	for _, tableName := range listOutput.TableNames {
		descOutput, err := describeAPI.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			continue // skip tables we can't describe (e.g. permission denied)
		}

		table := descOutput.Table

		name := ""
		if table.TableName != nil {
			name = *table.TableName
		}

		status := string(table.TableStatus)

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

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"table_name":   name,
				"status":       status,
				"item_count":   itemCount,
				"size_bytes":   sizeBytes,
				"billing_mode": billingMode,
			},
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
	}, nil
}
