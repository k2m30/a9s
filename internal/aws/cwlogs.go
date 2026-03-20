package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("logs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudWatchLogGroups(ctx, c.CloudWatchLogs)
	})
	resource.RegisterFieldKeys("logs", []string{"log_group_name", "stored_bytes", "retention_days", "creation_time"})
}

// FetchCloudWatchLogGroups calls the CloudWatchLogs DescribeLogGroups API and converts the
// response into a slice of generic Resource structs.
func FetchCloudWatchLogGroups(ctx context.Context, api CWLogsDescribeLogGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching CloudWatch log groups: %w", err)
	}

	var resources []resource.Resource

	for _, lg := range output.LogGroups {
		logGroupName := ""
		if lg.LogGroupName != nil {
			logGroupName = *lg.LogGroupName
		}

		storedBytes := ""
		if lg.StoredBytes != nil {
			storedBytes = fmt.Sprintf("%d", *lg.StoredBytes)
		}

		retentionDays := ""
		if lg.RetentionInDays != nil {
			retentionDays = fmt.Sprintf("%d", *lg.RetentionInDays)
		}

		creationTime := ""
		if lg.CreationTime != nil {
			creationTime = fmt.Sprintf("%d", *lg.CreationTime)
		}

		r := resource.Resource{
			ID:     logGroupName,
			Name:   logGroupName,
			Status: "",
			Fields: map[string]string{
				"log_group_name": logGroupName,
				"stored_bytes":   storedBytes,
				"retention_days": retentionDays,
				"creation_time":  creationTime,
			},
			RawStruct:  lg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
