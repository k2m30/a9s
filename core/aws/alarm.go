// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCloudWatchAlarmsPage calls the CloudWatch DescribeAlarms API and returns
// a single page of alarms. Pass an empty continuationToken for the first page.
func FetchCloudWatchAlarmsPage(ctx context.Context, api CloudWatchDescribeAlarmsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatch.DescribeAlarmsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAlarms(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudWatch alarms: %w", err)
	}

	var resources []resource.Resource
	for _, alarm := range output.MetricAlarms {
		alarmName := ""
		if alarm.AlarmName != nil {
			alarmName = *alarm.AlarmName
		}

		stateValue := string(alarm.StateValue)

		metricName := ""
		if alarm.MetricName != nil {
			metricName = *alarm.MetricName
		}

		namespace := ""
		if alarm.Namespace != nil {
			namespace = *alarm.Namespace
		}

		threshold := ""
		if alarm.Threshold != nil {
			threshold = fmt.Sprintf("%.2f", *alarm.Threshold)
		}

		actionsCount := len(alarm.AlarmActions)

		r := resource.Resource{
			ID:       alarmName,
			Name:     alarmName,
			Findings: alarmStateFindings(stateValue, actionsCount),
			Fields: map[string]string{
				"alarm_name":    alarmName,
				"state":         stateValue,
				"metric_name":   metricName,
				"namespace":     namespace,
				"threshold":     threshold,
				"actions_count": strconv.Itoa(actionsCount),
			},
			RawStruct: alarm,
		}

		// Independent of the state/no-actions pair: an alarm can have actions
		// wired and still have execution switched off, and it reads as healthy
		// in the console. Nil means DescribeAlarms did not resolve the switch,
		// which is not evidence that it is off.
		if alarm.ActionsEnabled != nil && !*alarm.ActionsEnabled {
			addWave1Finding(&r, CodeAlarmActionsDisabled, domain.SevWarn)
			// The phrase already says the actions are off; the row says how
			// much is wired behind the switch.
			addWave1Rows(&r, CodeAlarmActionsDisabled, domain.DetailRow{
				Label: "Configured actions", Value: strconv.Itoa(actionsCount), Tier: "~",
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

func alarmStateFindings(state string, actionsCount int) []domain.Finding {
	switch state {
	case "ALARM":
		return []domain.Finding{wave1Finding(CodeAlarmStateAlarm, domain.SevBroken)}
	case "INSUFFICIENT_DATA":
		return []domain.Finding{wave1Finding(CodeAlarmStateInsufficient, domain.SevWarn)}
	case "OK":
		if actionsCount == 0 {
			return []domain.Finding{wave1Finding(CodeAlarmNoActions, domain.SevWarn)}
		}
	}
	return nil
}
