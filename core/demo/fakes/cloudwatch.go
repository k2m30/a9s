// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// CloudWatchFake implements aws.CloudWatchAPI against fixture data loaded at construction time.
type CloudWatchFake struct {
	fix *fixtures.CloudWatchFixtures
}

// NewCloudWatch constructs a CloudWatchFake backed by fixture data from the fixtures package.
func NewCloudWatch() *CloudWatchFake {
	return &CloudWatchFake{fix: fixtures.NewCloudWatchFixtures()}
}

func (f *CloudWatchFake) DescribeAlarms(_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
	return &cloudwatch.DescribeAlarmsOutput{MetricAlarms: f.fix.Alarms}, nil
}

func (f *CloudWatchFake) DescribeAlarmHistory(_ context.Context, in *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	if in == nil || in.AlarmName == nil {
		return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: []cwtypes.AlarmHistoryItem{}}, nil
	}
	if !f.hasAlarm(*in.AlarmName) {
		return nil, &cwtypes.ResourceNotFound{Message: notFoundMessage("Alarm", *in.AlarmName)}
	}
	// A registered alarm with no history answers empty: "this alarm has never
	// changed state" and "there is no such alarm" are different facts.
	return &cloudwatch.DescribeAlarmHistoryOutput{
		AlarmHistoryItems: f.fix.AlarmHistory[*in.AlarmName],
	}, nil
}

// hasAlarm reports whether the fixtures register this alarm.
func (f *CloudWatchFake) hasAlarm(name string) bool {
	return slices.ContainsFunc(f.fix.Alarms, func(a cwtypes.MetricAlarm) bool {
		return aws.ToString(a.AlarmName) == name
	})
}
