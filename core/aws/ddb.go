// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeDDBFindings returns the findings for a DynamoDB table plus the
// supporting AttentionDetail rows keyed by the finding that owns them.
func computeDDBFindings(table *ddbtypes.TableDescription) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	if table == nil {
		return nil, nil
	}
	var posture []domain.Finding
	if !aws.ToBool(table.DeletionProtectionEnabled) {
		posture = append(posture, wave1Finding(CodeDDBDeletionProtectionOff))
	}
	return withState("ddb", string(table.TableStatus), posture), nil
}

func ddbTableResource(table *ddbtypes.TableDescription) resource.Resource {
	name := ""
	if table.TableName != nil {
		name = *table.TableName
	}

	findings, attentionDetails := computeDDBFindings(table)

	itemCount := ""
	if table.ItemCount != nil {
		itemCount = fmt.Sprintf("%d", *table.ItemCount)
	}

	sizeBytes, sizeBytesRaw := "", ""
	if table.TableSizeBytes != nil {
		sizeBytes = formatBytes(*table.TableSizeBytes)
		sizeBytesRaw = strconv.FormatInt(*table.TableSizeBytes, 10)
	}

	// DynamoDB omits BillingModeSummary entirely for a table that has
	// never switched mode, and PROVISIONED is the mode a table gets when
	// CreateTable names none.
	billingMode := domain.HumanizeStatusPhrase(string(ddbtypes.BillingModeProvisioned))
	if table.BillingModeSummary != nil {
		billingMode = domain.HumanizeStatusPhrase(string(table.BillingModeSummary.BillingMode))
	}

	arn := ""
	if table.TableArn != nil {
		arn = *table.TableArn
	}

	r := withWave1("ddb", resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"table_name":     name,
			"status":         string(table.TableStatus),
			"item_count":     itemCount,
			"size_bytes":     sizeBytes,
			"size_bytes_raw": sizeBytesRaw,
			"billing_mode":   billingMode,
			"arn":            arn,
		},
		RawStruct:        table,
		AttentionDetails: attentionDetails,
	}, findings...)
	return r
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

	var tally describeTally
	var resources []resource.Resource
	for _, tableName := range listOutput.TableNames {
		rows := tally.one("DescribeTable", tableName, tableName, func() (*resource.Resource, error) {
			out, err := describeAPI.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
			if err != nil {
				return nil, err
			}
			if out.Table == nil {
				return nil, UnusableAnswerErr{Call: "DescribeTable", Field: "table"}
			}
			r := ddbTableResource(out.Table)
			return &r, nil
		}, func(err error) resource.Resource { return DegradedDetails("ddb", tableName, err) })
		resources = append(resources, rows...)
	}

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
	}, tally.err("ddb: DescribeTable")
}
