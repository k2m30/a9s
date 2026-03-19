package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("alarm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudWatchAlarms(ctx, c.CloudWatch)
	})
}

// FetchCloudWatchAlarms calls the CloudWatch DescribeAlarms API and converts the
// response into a slice of generic Resource structs.
func FetchCloudWatchAlarms(ctx context.Context, api CloudWatchDescribeAlarmsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		return nil, err
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

		comparison := string(alarm.ComparisonOperator)

		detail := map[string]string{
			"Alarm Name":  alarmName,
			"State":       stateValue,
			"Metric Name": metricName,
			"Namespace":   namespace,
			"Threshold":   threshold,
			"Comparison":  comparison,
			"Statistic":   string(alarm.Statistic),
		}

		if alarm.AlarmArn != nil {
			detail["ARN"] = *alarm.AlarmArn
		}

		if alarm.AlarmDescription != nil {
			detail["Description"] = *alarm.AlarmDescription
		}

		if alarm.Period != nil {
			detail["Period"] = fmt.Sprintf("%d", *alarm.Period)
		}

		if alarm.EvaluationPeriods != nil {
			detail["Evaluation Periods"] = fmt.Sprintf("%d", *alarm.EvaluationPeriods)
		}

		if alarm.StateReason != nil {
			detail["State Reason"] = *alarm.StateReason
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(alarm, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     alarmName,
			Name:   alarmName,
			Status: stateValue,
			Fields: map[string]string{
				"alarm_name":  alarmName,
				"state":       stateValue,
				"metric_name": metricName,
				"namespace":   namespace,
				"threshold":   threshold,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  alarm,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
