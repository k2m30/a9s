// t565_demo_log_order_test.go — the demo CloudWatch Logs fake answers
// GetLogEvents the way AWS does: events in timestamp order.
package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

func TestDemoGetLogEvents_AnswersInTimestampOrder(t *testing.T) {
	api := fakes.NewCWLogs()
	for group := range fixtures.NewCWLogsFixtures().LogEvents {
		out, err := api.GetLogEvents(context.Background(), &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(group),
			LogStreamName: aws.String("any"),
		})
		if err != nil {
			t.Fatalf("%s: %v", group, err)
		}
		if !slices.IsSortedFunc(out.Events, func(a, b cwlogstypes.OutputLogEvent) int {
			return int(aws.ToInt64(a.Timestamp) - aws.ToInt64(b.Timestamp))
		}) {
			t.Errorf("%s: GetLogEvents answers out of timestamp order", group)
		}
	}
}
