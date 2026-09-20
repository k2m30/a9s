package unit_test

// logs_alarm_bridge_test.go — the log group ↔ alarm metric-filter bridge.
//
// A metric filter turns a log pattern into a CloudWatch metric in a namespace
// the operator chose, and the alarm is over that metric: nothing on the alarm
// names the log group and nothing on the log group names the alarm. One
// logs:DescribeMetricFilters per open is what links them, from either end.

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func logsAlarmBridgeCache(t *testing.T, alarms []resource.Resource) resource.ResourceCache {
	t.Helper()
	cache := rel2DemoCacheFor(t, "logs")
	cache["alarm"] = resource.ResourceCacheEntry{Resources: alarms}
	return cache
}

func logsAlarmMetricAlarm(id, namespace, metric string, dims ...cwtypes.Dimension) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:          aws.String(id),
			Namespace:          aws.String(namespace),
			MetricName:         aws.String(metric),
			Dimensions:         dims,
			StateValue:         cwtypes.StateValueOk,
			ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
			Threshold:          aws.Float64(20),
			EvaluationPeriods:  aws.Int32(1),
			ActionsEnabled:     aws.Bool(true),
		},
	}
}

func TestLogGroupAndAlarmFindEachOtherThroughTheMetricFilter(t *testing.T) {
	group := alarmMatchDemoRow(t, "logs", alarmMatchByName(fixtures.OrphanOldLogGroupName))
	onFilterMetric := logsAlarmMetricAlarm("orphan-old-errors", fixtures.OrphanOldMetricNamespace, fixtures.OrphanOldMetricName)
	sameMetricOtherNamespace := logsAlarmMetricAlarm("orphan-old-errors-elsewhere", "Other/Namespace", fixtures.OrphanOldMetricName)
	otherMetric := logsAlarmMetricAlarm("unrelated-count", fixtures.OrphanOldMetricNamespace, "SomeOtherCount")
	cache := logsAlarmBridgeCache(t, []resource.Resource{onFilterMetric, sameMetricOtherNamespace, otherMetric})
	clients := demo.NewServiceClients()

	forward := rel2CheckerFor(t, "logs", "alarm")(context.Background(), clients, group, cache).ResourceIDs()
	if !slices.Equal(forward, []string{onFilterMetric.ID}) {
		t.Errorf("logs %s → alarms = %v, want [%s]: its metric filter emits the metric that alarm watches", group.ID, forward, onFilterMetric.ID)
	}

	reverse := rel2CheckerFor(t, "alarm", "logs")(context.Background(), clients, onFilterMetric, cache).ResourceIDs()
	if !slices.Equal(reverse, []string{group.ID}) {
		t.Errorf("alarm %s → logs = %v, want [%s]", onFilterMetric.ID, reverse, group.ID)
	}
	for _, unrelated := range []resource.Resource{sameMetricOtherNamespace, otherMetric} {
		if got := rel2CheckerFor(t, "alarm", "logs")(context.Background(), clients, unrelated, cache).ResourceIDs(); len(got) != 0 {
			t.Errorf("alarm %s → logs = %v, want none: no metric filter emits that metric", unrelated.ID, got)
		}
	}
}

// A log group nothing filters, and an alarm over a metric no filter emits,
// each answer a proven zero rather than a guess.
func TestLogGroupWithNoMetricFilterClaimsNoAlarm(t *testing.T) {
	var unfiltered resource.Resource
	for _, row := range rel2DemoList(t, "logs") {
		if row.ID != fixtures.OrphanOldLogGroupName {
			unfiltered = row
			break
		}
	}
	onFilterMetric := logsAlarmMetricAlarm("orphan-old-errors", fixtures.OrphanOldMetricNamespace, fixtures.OrphanOldMetricName)
	cache := logsAlarmBridgeCache(t, []resource.Resource{onFilterMetric})

	got := rel2CheckerFor(t, "logs", "alarm")(context.Background(), demo.NewServiceClients(), unfiltered, cache)
	if got.Count() != 0 {
		t.Errorf("logs %s → alarms = %v, want none", unfiltered.ID, got.ResourceIDs())
	}
	if got.Truncated() {
		t.Errorf("logs %s → alarms is a lower bound, want a proven zero: the filters were read in full", unfiltered.ID)
	}
}

// An alarm over a metric-math expression watches several metrics and names
// none of them at the top level, so there is no one metric to look a filter
// up by; the dimension arm still answers for it.
func TestMetricMathAlarmHasNoMetricToBridgeWith(t *testing.T) {
	group := alarmMatchDemoRow(t, "logs", alarmMatchByName(fixtures.OrphanOldLogGroupName))
	math := alarmMatchResource("orphan-old-error-ratio", alarmMatchMetricMath("AWS/Logs", "LogGroupName", group.ID))
	cache := logsAlarmBridgeCache(t, []resource.Resource{math})
	clients := demo.NewServiceClients()

	if _, named := awsclient.AlarmMetricWatched(math.RawStruct.(cwtypes.MetricAlarm)); named {
		t.Fatal("a metric-math alarm names no top-level metric")
	}
	if got := rel2CheckerFor(t, "logs", "alarm")(context.Background(), clients, group, cache).ResourceIDs(); !slices.Equal(got, []string{math.ID}) {
		t.Errorf("logs %s → alarms = %v, want [%s]: the alarm carries the group as a dimension", group.ID, got, math.ID)
	}
}
