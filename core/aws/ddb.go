// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeDDBFindings returns the findings for a DynamoDB table plus the
// supporting AttentionDetail rows keyed by the finding that owns them. The
// lifecycle status owns the status column; the deletion-protection posture
// row is evaluated independently and stacks on top of it, except on a table
// that is already on its way out.
func computeDDBFindings(table *ddbtypes.TableDescription) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	if table == nil {
		return nil, nil
	}
	var lifecycle []domain.Finding
	switch table.TableStatus {
	case ddbtypes.TableStatusActive:
	case ddbtypes.TableStatusInaccessibleEncryptionCredentials:
		lifecycle = []domain.Finding{{Code: CodeDDBKMSKeyInaccessible, Phrase: "kms key inaccessible", Severity: domain.SevBroken, Source: "wave1"}}
	case ddbtypes.TableStatusArchived:
		lifecycle = []domain.Finding{{Code: CodeDDBArchivedKMSLost, Phrase: "archived: kms key lost", Severity: domain.SevBroken, Source: "wave1"}}
	case ddbtypes.TableStatusCreating:
		lifecycle = []domain.Finding{{Code: CodeDDBCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusUpdating:
		lifecycle = []domain.Finding{{Code: CodeDDBUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusDeleting:
		lifecycle = []domain.Finding{{Code: CodeDDBDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	case ddbtypes.TableStatusArchiving:
		lifecycle = []domain.Finding{{Code: CodeDDBArchiving, Phrase: "archiving", Severity: domain.SevWarn, Source: "wave1"}}
	}

	if resourceIsTearingDown(table) {
		return lifecycle, nil
	}
	if aws.ToBool(table.DeletionProtectionEnabled) {
		return lifecycle, nil
	}
	return append(lifecycle, domain.Finding{
		Code:     CodeDDBDeletionProtectionOff,
		Phrase:   "deletion protection off",
		Detail:   catalog.Detail(CodeDDBDeletionProtectionOff),
		Severity: domain.SevWarn,
		Source:   "wave1",
	}), nil
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
			// Surface per-table failures to the error log AND keep the row —
			// a listed table must never vanish behind a describe denial.
			failures = append(failures, fmt.Sprintf("%s: %v", tableName, err))
			resources = append(resources, DegradedDetails("ddb", tableName, err))
			continue
		}

		table := descOutput.Table
		if table == nil {
			failures = append(failures, fmt.Sprintf("%s: nil table in response", tableName))
			resources = append(resources, DegradedDetails("ddb", tableName, nil))
			continue
		}

		name := ""
		if table.TableName != nil {
			name = *table.TableName
		}

		findings, attentionDetails := computeDDBFindings(table)
		statusPhrase := domain.StatusPhrase(findings)

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
			billingMode = domain.HumanizeStatusPhrase(string(table.BillingModeSummary.BillingMode))
		}

		arn := ""
		if table.TableArn != nil {
			arn = *table.TableArn
		}

		r := resource.Resource{
			ID:       name,
			Name:     name,
			Findings: findings,
			Fields: map[string]string{
				"table_name":   name,
				"status":       statusPhrase,
				"item_count":   itemCount,
				"size_bytes":   sizeBytes,
				"billing_mode": billingMode,
				"arn":          arn,
			},
			RawStruct:        table,
			AttentionDetails: attentionDetails,
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
