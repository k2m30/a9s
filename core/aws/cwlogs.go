// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// logsCodeRetentionNeverExpire is the canonical FindingCode for a log group
// with no retention policy (RetentionInDays == nil), meaning events are kept
// forever and billed indefinitely.
const logsCodeRetentionNeverExpire domain.FindingCode = "logs.retention-never-expire"

// logsCodeStaleEmpty is the canonical FindingCode for a log group that is
// empty (StoredBytes == 0) and was created more than logsStaleEmptyAfterDays ago — likely an
// orphaned log group no application still writes to.
const logsCodeStaleEmpty domain.FindingCode = "logs.stale-empty"

// CodeLogsNoKMS is the canonical FindingCode for a log group with no customer
// managed KMS key (KmsKeyId empty).
const CodeLogsNoKMS domain.FindingCode = "logs.no-kms"

// logsFullFeatureClasses are the log classes that carry metric filters and
// answer GetLogEvents. AWS, "Log classes": metric filters are Standard only,
// GetLogEvents and FilterLogEvents are "Not supported" on Infrequent Access
// (read-path APIs return ESSENTIALS for it from December 2026), and the
// Delivery class "doesn't offer rich CloudWatch Logs capabilities". A class
// AWS adds later is outside the list until it is known to support them.
var logsFullFeatureClasses = []string{string(cwlogstypes.LogGroupClassStandard)} //nolint:gochecknoglobals // static AWS value set

// logsAWSRetentionClasses are the log classes whose retention AWS sets and the
// customer cannot change ("Log classes": the Delivery class retention "is fixed
// and cannot be changed"); DescribeLogGroups reports no retention for them.
var logsAWSRetentionClasses = []string{string(cwlogstypes.LogGroupClassDelivery)} //nolint:gochecknoglobals // static AWS value set

// logsKind is the group's log class. DescribeLogGroups reports no class on a
// group created before classes existed, and such a group is Standard.
func logsKind(r domain.Resource) string {
	class := r.Fields["log_group_class"]
	if lg, ok := assertStruct[cwlogstypes.LogGroup](r.RawStruct); ok {
		class = string(lg.LogGroupClass)
	}
	if class == "" {
		return string(cwlogstypes.LogGroupClassStandard)
	}
	return class
}

// logsOffersEvents gates the drill into a group's streams and their events,
// which read GetLogEvents.
func logsOffersEvents(r domain.Resource) bool {
	return slices.Contains(logsFullFeatureClasses, logsKind(r))
}

// The words the fetcher writes into Fields for the two settings a log group is
// judged on. logsGroupFindings reads these rather than the SDK struct, so a row
// rebuilt from Fields alone reaches the same verdict, and a row carrying
// neither word — or a word from no vocabulary — is reported on for neither.
const (
	logsRetentionNeverExpires = "never expire"
	logsRetentionFixedByAWS   = "fixed by AWS"

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
		out = append(out, wave1Finding(logsCodeRetentionNeverExpire))
	case storedBytes == "0 B" && olderThan(creationTime, days(logsStaleEmptyAfterDays)):
		out = append(out, wave1Finding(logsCodeStaleEmpty, strconv.Itoa(logsStaleEmptyAfterDays)))
	}
	if encryption == logsEncryptionNone {
		out = append(out, wave1Finding(CodeLogsNoKMS))
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

		creationTime := ""
		if lg.CreationTime != nil {
			creationTime = formatEpochMillis(*lg.CreationTime)
		}

		kmsKeyID := ""
		if lg.KmsKeyId != nil {
			kmsKeyID = *lg.KmsKeyId
		}

		retention := logsRetentionNeverExpires
		switch {
		case lg.RetentionInDays != nil:
			retention = fmt.Sprintf("%d days", *lg.RetentionInDays)
		case slices.Contains(logsAWSRetentionClasses, logsKind(domain.Resource{RawStruct: lg})):
			retention = logsRetentionFixedByAWS
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
				"creation_time":    creationTime,
				"kms_key_id":       kmsKeyID,
				"retention":        retention,
				"encryption":       encryption,
				"log_group_class":  string(lg.LogGroupClass),
			},
			RawStruct: lg,
		}

		r = withWave1(r, logsGroupFindings(retention, storedBytes, creationTime, encryption)...)
		if encryption == logsEncryptionNone {
			addWave1Rows(&r, CodeLogsNoKMS, domain.DetailRow{
				Label: "KMS key", Value: "none", Tier: "~",
			})
		}

		resources = append(resources, r)
	}

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
