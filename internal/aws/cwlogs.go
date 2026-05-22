package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchCloudWatchLogGroups calls the CloudWatchLogs DescribeLogGroups API and
// returns all pages of log groups. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchCloudWatchLogGroups(ctx context.Context, api CWLogsDescribeLogGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCloudWatchLogGroupsPage(ctx, api, token)
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
			ID:    logGroupName,
			Name:  logGroupName,
			Fields: map[string]string{
				"log_group_name": logGroupName,
				"stored_bytes":   storedBytes,
				"retention_days": retentionDays,
				"creation_time":  creationTime,
				"kms_key_id":     kmsKeyID,
			},
			RawStruct: lg,
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
