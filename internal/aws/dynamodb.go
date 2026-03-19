package aws

import (
	"context"
	"encoding/json"
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
}

// FetchDynamoDBTables performs a two-step fetch: ListTables to get names,
// then DescribeTable per table for full details.
func FetchDynamoDBTables(ctx context.Context, listAPI DDBListTablesAPI, describeAPI DDBDescribeTableAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return nil, err
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

		detail := map[string]string{
			"Table Name":   name,
			"Status":       status,
			"Item Count":   itemCount,
			"Size (bytes)": sizeBytes,
			"Billing Mode": billingMode,
		}

		if table.TableArn != nil {
			detail["ARN"] = *table.TableArn
		}

		if table.CreationDateTime != nil {
			detail["Created"] = table.CreationDateTime.Format("2006-01-02T15:04:05Z07:00")
		}

		if table.TableId != nil {
			detail["Table ID"] = *table.TableId
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(table, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  table,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
