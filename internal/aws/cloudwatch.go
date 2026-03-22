package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("alarm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudWatchAlarms(ctx, c.CloudWatch)
	})
	resource.RegisterFieldKeys("alarm", []string{"alarm_name", "state", "metric_name", "namespace", "threshold"})
}

// FetchCloudWatchAlarms calls the CloudWatch DescribeAlarms API and converts the
// response into a slice of generic Resource structs.
func FetchCloudWatchAlarms(ctx context.Context, api CloudWatchDescribeAlarmsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching CloudWatch alarms: %w", err)
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
			RawStruct:  alarm,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
