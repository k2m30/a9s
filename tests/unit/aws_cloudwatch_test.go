package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// float64Ptr returns a pointer to the given float64 value.
func float64Ptr(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// CloudWatch Alarms fetcher tests
// ---------------------------------------------------------------------------

func TestFetchCloudWatchAlarms_ParsesMultipleAlarms(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsClient{
		output: &cloudwatch.DescribeAlarmsOutput{
			MetricAlarms: []cwtypes.MetricAlarm{
				{
					AlarmName:          aws.String("high-cpu-alarm"),
					StateValue:         cwtypes.StateValueAlarm,
					MetricName:         aws.String("CPUUtilization"),
					Namespace:          aws.String("AWS/EC2"),
					Threshold:          float64Ptr(80.0),
					ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
					Statistic:          cwtypes.StatisticAverage,
					AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:high-cpu-alarm"),
					AlarmDescription:   aws.String("Alarm when CPU exceeds 80%"),
					Period:             aws.Int32(300),
					EvaluationPeriods:  aws.Int32(2),
					StateReason:        aws.String("Threshold crossed: 1 datapoint"),
				},
				{
					AlarmName:          aws.String("low-disk-alarm"),
					StateValue:         cwtypes.StateValueOk,
					MetricName:         aws.String("DiskReadOps"),
					Namespace:          aws.String("AWS/EC2"),
					Threshold:          float64Ptr(100.0),
					ComparisonOperator: cwtypes.ComparisonOperatorLessThanThreshold,
					Statistic:          cwtypes.StatisticSum,
					AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:low-disk-alarm"),
					AlarmDescription:   aws.String("Alarm when disk ops fall below 100"),
					Period:             aws.Int32(60),
					EvaluationPeriods:  aws.Int32(1),
					StateReason:        aws.String("Threshold not crossed"),
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudWatchAlarms(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"alarm_name", "state", "metric_name", "namespace", "threshold"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first alarm
	r0 := resources[0]
	if r0.ID != "high-cpu-alarm" {
		t.Errorf("resource[0].ID: expected %q, got %q", "high-cpu-alarm", r0.ID)
	}
	if r0.Name != "high-cpu-alarm" {
		t.Errorf("resource[0].Name: expected %q, got %q", "high-cpu-alarm", r0.Name)
	}
	if r0.Status != "ALARM" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ALARM", r0.Status)
	}
	if r0.Fields["alarm_name"] != "high-cpu-alarm" {
		t.Errorf("resource[0].Fields[\"alarm_name\"]: expected %q, got %q", "high-cpu-alarm", r0.Fields["alarm_name"])
	}
	if r0.Fields["state"] != "ALARM" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "ALARM", r0.Fields["state"])
	}
	if r0.Fields["metric_name"] != "CPUUtilization" {
		t.Errorf("resource[0].Fields[\"metric_name\"]: expected %q, got %q", "CPUUtilization", r0.Fields["metric_name"])
	}
	if r0.Fields["namespace"] != "AWS/EC2" {
		t.Errorf("resource[0].Fields[\"namespace\"]: expected %q, got %q", "AWS/EC2", r0.Fields["namespace"])
	}
	if r0.Fields["threshold"] != "80.00" {
		t.Errorf("resource[0].Fields[\"threshold\"]: expected %q, got %q", "80.00", r0.Fields["threshold"])
	}

	// Verify second alarm
	r1 := resources[1]
	if r1.ID != "low-disk-alarm" {
		t.Errorf("resource[1].ID: expected %q, got %q", "low-disk-alarm", r1.ID)
	}
	if r1.Status != "OK" {
		t.Errorf("resource[1].Status: expected %q, got %q", "OK", r1.Status)
	}
	if r1.Fields["threshold"] != "100.00" {
		t.Errorf("resource[1].Fields[\"threshold\"]: expected %q, got %q", "100.00", r1.Fields["threshold"])
	}
}

func TestFetchCloudWatchAlarms_ErrorResponse(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCloudWatchAlarms(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchCloudWatchAlarms_EmptyResponse(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsClient{
		output: &cloudwatch.DescribeAlarmsOutput{
			MetricAlarms: []cwtypes.MetricAlarm{},
		},
	}

	resources, err := awsclient.FetchCloudWatchAlarms(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
