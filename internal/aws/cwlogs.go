package aws

import (
	"context"
	"encoding/json"
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
}

// FetchCloudWatchLogGroups calls the CloudWatchLogs DescribeLogGroups API and converts the
// response into a slice of generic Resource structs.
func FetchCloudWatchLogGroups(ctx context.Context, api CWLogsDescribeLogGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{})
	if err != nil {
		return nil, err
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

		detail := map[string]string{
			"Log Group Name":  logGroupName,
			"Stored Bytes":    storedBytes,
			"Retention (days)": retentionDays,
			"Creation Time":   creationTime,
		}

		if lg.Arn != nil {
			detail["ARN"] = *lg.Arn
		}

		if lg.KmsKeyId != nil {
			detail["KMS Key ID"] = *lg.KmsKeyId
		}

		detail["Data Protection"] = string(lg.DataProtectionStatus)

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(lg, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  lg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
