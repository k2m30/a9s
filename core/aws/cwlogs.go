// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

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

// CodeLogsNoKMS is the canonical FindingCode for a log group with no customer
// managed KMS key (KmsKeyId empty). docs/resources/logs.md §4.
const CodeLogsNoKMS domain.FindingCode = "logs.no-kms"

// logsStaleEmptyAge is the age threshold colorLogs uses to flag an empty log
// group as stale.
const logsStaleEmptyAge = 90 * 24 * time.Hour

// The words the fetcher writes into Fields for the two settings a log group is
// judged on. logsGroupFindings reads these rather than the SDK struct, so a row
// rebuilt from Fields alone reaches the same verdict, and a row carrying
// neither word — or a word from no vocabulary — is reported on for neither.
const (
	logsRetentionNeverExpires = "never expire"
	logsRetentionExpires      = "expires"

	logsEncryptionKMS  = "kms"
	logsEncryptionNone = "none"
)

// logsGroupFindings is the one predicate for a log group. Never-expiring
// retention wins over the stale-empty check: a group nobody writes to and
// nobody expires is first a retention problem. Encryption is independent of
// both — a group can be unencrypted and never-expiring, and fixing one leaves
// the other.
func logsGroupFindings(retention, storedBytes, creationTime, encryption string) []domain.Finding {
	var out []domain.Finding
	switch {
	case retention == logsRetentionNeverExpires:
		out = append(out, wave1Finding(logsCodeRetentionNeverExpire, domain.SevWarn))
	case storedBytes == "0 B" && olderThan(creationTime, logsStaleEmptyAge):
		out = append(out, wave1Finding(logsCodeStaleEmpty, domain.SevWarn))
	}
	if encryption == logsEncryptionNone {
		out = append(out, wave1Finding(CodeLogsNoKMS, domain.SevWarn))
	}
	return out
}

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

		storedBytes, storedBytesRaw := "", ""
		if lg.StoredBytes != nil {
			storedBytes = formatBytes(*lg.StoredBytes)
			storedBytesRaw = strconv.FormatInt(*lg.StoredBytes, 10)
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

		retention := logsRetentionExpires
		if lg.RetentionInDays == nil {
			retention = logsRetentionNeverExpires
		}
		encryption := logsEncryptionNone
		if kmsKeyID != "" {
			encryption = logsEncryptionKMS
		}

		r := resource.Resource{
			ID:   logGroupName,
			Name: logGroupName,
			Fields: map[string]string{
				"log_group_name":   logGroupName,
				"stored_bytes":     storedBytes,
				"stored_bytes_raw": storedBytesRaw,
				"retention_days":   retentionDays,
				"creation_time":    creationTime,
				"kms_key_id":       kmsKeyID,
				"retention":        retention,
				"encryption":       encryption,
			},
			RawStruct: lg,
		}

		r.Findings = logsGroupFindings(retention, storedBytes, creationTime, encryption)
		if encryption == logsEncryptionNone {
			addWave1Rows(&r, CodeLogsNoKMS, domain.DetailRow{
				Label: "KMS key", Value: "none", Tier: "~",
			})
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
