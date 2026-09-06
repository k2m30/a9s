// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// logsCodeRetentionNeverExpire is the canonical FindingCode for a log group
// with no retention policy (RetentionInDays == nil), meaning events are kept
// forever and billed indefinitely. docs/resources/logs.md §4.
const logsCodeRetentionNeverExpire domain.FindingCode = "logs.retention-never-expire"

// logsCodeStaleEmpty is the canonical FindingCode for a log group that is
// empty (StoredBytes == 0) and was created more than 90 days ago — likely an
// orphaned log group no application still writes to.
const logsCodeStaleEmpty domain.FindingCode = "logs.stale-empty"

// logsStaleEmptyAge is the age threshold colorLogs uses to flag an empty log
// group as stale.
const logsStaleEmptyAge = 90 * 24 * time.Hour

// FetchCloudWatchLogGroupsPage calls the CloudWatchLogs DescribeLogGroups API and returns
// a single page of log groups. Pass an empty continuationToken for the first page.
func FetchCloudWatchLogGroupsPage(ctx context.Context, api CWLogsDescribeLogGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		Limit: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeLogGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudWatch log groups: %w", err)
	}

	var resources []resource.Resource
	for _, lg := range output.LogGroups {
		logGroupName := ""
		if lg.LogGroupName != nil {
			logGroupName = *lg.LogGroupName
		}

		storedBytes := ""
		if lg.StoredBytes != nil {
			storedBytes = formatBytes(*lg.StoredBytes)
		}

		retentionDays := ""
		if lg.RetentionInDays != nil {
			retentionDays = fmt.Sprintf("%d", *lg.RetentionInDays)
		}

		creationTime := ""
		if lg.CreationTime != nil {
			creationTime = formatEpochMillis(*lg.CreationTime)
		}

		kmsKeyID := ""
		if lg.KmsKeyId != nil {
			kmsKeyID = *lg.KmsKeyId
		}

		r := resource.Resource{
			ID:   logGroupName,
			Name: logGroupName,
			Fields: map[string]string{
				"log_group_name": logGroupName,
				"stored_bytes":   storedBytes,
				"retention_days": retentionDays,
				"creation_time":  creationTime,
				"kms_key_id":     kmsKeyID,
			},
			RawStruct: lg,
		}

		// Wave-1 classification mirrors colorLogs's own precedence:
		// RetentionInDays == nil (never expires, billed indefinitely) wins
		// first; only when retention IS set do we flag an empty log group
		// that has sat unwritten for 90+ days (likely orphaned).
		switch {
		case lg.RetentionInDays == nil:
			r.Findings = []domain.Finding{{
				Code:     logsCodeRetentionNeverExpire,
				Phrase:   "retention: never expire",
				Detail:   catalog.Detail(logsCodeRetentionNeverExpire),
				Severity: domain.SevWarn,
				Source:   "wave1",
			}}
		case lg.StoredBytes != nil && *lg.StoredBytes == 0 && lg.CreationTime != nil && time.Since(time.UnixMilli(*lg.CreationTime)) > logsStaleEmptyAge:
			r.Findings = []domain.Finding{{
				Code:     logsCodeStaleEmpty,
				Phrase:   "empty, created over 90 days ago",
				Severity: domain.SevWarn,
				Source:   "wave1",
			}}
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
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
