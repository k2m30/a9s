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
					Threshold:          new(80.0),
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
					Threshold:          new(100.0),
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

// TestFetchCloudWatchAlarms_ActionsCount_AlarmActionsOnly verifies that the
// actions_count field counts only AlarmActions, not OKActions or
// InsufficientDataActions.
//
// CodeRabbit PR-273 finding: internal/aws/alarm.go sums all three
// action slices, but docs/attention-signals.md specifies the alarm attention
// signal keys off AlarmActions==[] only. Mixing in OKActions and
// InsufficientDataActions inflates the count and masks alarms with no real
// actions configured.
func TestFetchCloudWatchAlarms_ActionsCount_AlarmActionsOnly(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmsClient{
		output: &cloudwatch.DescribeAlarmsOutput{
			MetricAlarms: []cwtypes.MetricAlarm{
				{
					// Only AlarmActions populated — count must be 1.
					AlarmName:               aws.String("alarm-with-alarm-actions"),
					StateValue:              cwtypes.StateValueOk,
					AlarmActions:            []string{"arn:aws:sns:us-east-1:123456789012:my-topic"},
					OKActions:               []string{},
					InsufficientDataActions: []string{},
				},
				{
					// Only OKActions populated — actions_count must be "0"
					// because OKActions are not alarm-trigger actions.
					AlarmName:               aws.String("alarm-no-alarm-actions-but-ok-actions"),
					StateValue:              cwtypes.StateValueOk,
					AlarmActions:            []string{},
					OKActions:               []string{"arn:aws:sns:us-east-1:123456789012:ok-topic-1", "arn:aws:sns:us-east-1:123456789012:ok-topic-2"},
					InsufficientDataActions: []string{},
				},
				{
					// Only InsufficientDataActions populated — actions_count must be "0".
					AlarmName:               aws.String("alarm-only-insufficient-data-actions"),
					StateValue:              cwtypes.StateValueOk,
					AlarmActions:            []string{},
					OKActions:               []string{},
					InsufficientDataActions: []string{"arn:aws:sns:us-east-1:123456789012:insuf-topic"},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudWatchAlarms(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// alarm-with-alarm-actions: 1 AlarmAction → actions_count must be "1"
	r0 := resources[0]
	if r0.Fields["actions_count"] != "1" {
		t.Errorf("alarm-with-alarm-actions: actions_count = %q, want %q", r0.Fields["actions_count"], "1")
	}

	// alarm-no-alarm-actions-but-ok-actions: 0 AlarmActions → actions_count must be "0"
	// (NOT "2" which would result from counting OKActions too)
	r1 := resources[1]
	if r1.Fields["actions_count"] != "0" {
		t.Errorf("alarm-no-alarm-actions-but-ok-actions: actions_count = %q, want %q (OKActions must not be counted)", r1.Fields["actions_count"], "0")
	}

	// alarm-only-insufficient-data-actions: 0 AlarmActions → actions_count must be "0"
	// (NOT "1" which would result from counting InsufficientDataActions too)
	r2 := resources[2]
	if r2.Fields["actions_count"] != "0" {
		t.Errorf("alarm-only-insufficient-data-actions: actions_count = %q, want %q (InsufficientDataActions must not be counted)", r2.Fields["actions_count"], "0")
	}
}
