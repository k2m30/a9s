package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("ddb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDynamoDBTables(ctx, c.DynamoDB, c.DynamoDB)
	})
	resource.RegisterFieldKeys("ddb", []string{"table_name", "status", "item_count", "size_bytes", "billing_mode"})
}

// FetchDynamoDBTables performs a two-step fetch: ListTables to get names,
// then DescribeTable per table for full details.
func FetchDynamoDBTables(ctx context.Context, listAPI DDBListTablesAPI, describeAPI DDBDescribeTableAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return nil, fmt.Errorf("listing DynamoDB tables: %w", err)
	}

	var resources []resource.Resource

	for _, tableName := range listOutput.TableNames {
		descOutput, err := describeAPI.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			return nil, err
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
			sizeBytes = fmt.Sprintf("%d", *table.TableSizeBytes)
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
			RawStruct:  table,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
