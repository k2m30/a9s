// row_identity_events_test.go — row identity for event views.
//
// None of these APIs hands out an event ID, so a row's ID has to come from
// the event itself: its full-precision timestamp plus its content. A minute
// timestamp merges every event of that minute into one row; a position in the
// response gives the same event a new ID whenever a newer event shifts the
// window, so a refresh moves the cursor and the carried findings onto a
// different line.
package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Alarm history
// ---------------------------------------------------------------------------

type ridAlarmHistoryFake struct {
	items []cwtypes.AlarmHistoryItem
}

func (f *ridAlarmHistoryFake) DescribeAlarmHistory(_ context.Context, _ *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: f.items}, nil
}

func ridStateUpdate(at time.Time, from, to string) cwtypes.AlarmHistoryItem {
	return cwtypes.AlarmHistoryItem{
		AlarmName:       aws.String("acme-api-5xx"),
		AlarmType:       cwtypes.AlarmTypeMetricAlarm,
		HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
		HistorySummary:  aws.String("Alarm updated from " + from + " to " + to),
		HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"` + from + `"},"newState":{"stateValue":"` + to + `"}}`),
		Timestamp:       aws.Time(at),
	}
}

func ridAlarmHistoryRows(t *testing.T, items ...cwtypes.AlarmHistoryItem) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchAlarmHistory(context.Background(), &ridAlarmHistoryFake{items: items},
		map[string]string{"alarm_name": "acme-api-5xx"}, "")
	if err != nil {
		t.Fatalf("FetchAlarmHistory: %v", err)
	}
	if len(out.Resources) != len(items) {
		t.Fatalf("fetched %d rows, want %d", len(out.Resources), len(items))
	}
	return out.Resources
}

// TestRowIdentity_AlarmHistory_EventsInOneMinuteAreDistinct pins that an
// alarm that flaps OK -> ALARM -> OK inside one minute shows both transitions.
// The action item CloudWatch logs at the very millisecond of the transition
// is a third row, not a replacement for it.
func TestRowIdentity_AlarmHistory_EventsInOneMinuteAreDistinct(t *testing.T) {
	toAlarm := time.Date(2026, 9, 18, 10, 15, 7, 123_000_000, time.UTC)
	action := cwtypes.AlarmHistoryItem{
		AlarmName:       aws.String("acme-api-5xx"),
		AlarmType:       cwtypes.AlarmTypeMetricAlarm,
		HistoryItemType: cwtypes.HistoryItemTypeAction,
		HistorySummary:  aws.String("Successfully executed action arn:aws:sns:us-east-1:123456789012:acme-oncall"),
		HistoryData:     aws.String(`{"actionState":"Succeeded","notificationResource":"arn:aws:sns:us-east-1:123456789012:acme-oncall"}`),
		Timestamp:       aws.Time(toAlarm),
	}
	rows := ridAlarmHistoryRows(t,
		ridStateUpdate(time.Date(2026, 9, 18, 10, 15, 48, 456_000_000, time.UTC), "ALARM", "OK"),
		action,
		ridStateUpdate(toAlarm, "OK", "ALARM"),
	)
	ridAssertDistinctIDs(t, rows)
}

// TestRowIdentity_AlarmHistory_IDSurvivesANewerEvent: the same history item
// keeps its ID when a newer item arrives ahead of it in the next response.
func TestRowIdentity_AlarmHistory_IDSurvivesANewerEvent(t *testing.T) {
	older := ridStateUpdate(time.Date(2026, 9, 18, 10, 15, 7, 123_000_000, time.UTC), "OK", "ALARM")
	newer := ridStateUpdate(time.Date(2026, 9, 18, 10, 21, 30, 0, time.UTC), "ALARM", "OK")

	first := ridAlarmHistoryRows(t, older)
	second := ridAlarmHistoryRows(t, newer, older)
	if first[0].ID != second[1].ID {
		t.Errorf("the same history item has ID %q in one response and %q in the next", first[0].ID, second[1].ID)
	}
}

// ---------------------------------------------------------------------------
// RDS events
// ---------------------------------------------------------------------------

type ridRDSEventsFake struct {
	events []rdstypes.Event
}

func (f *ridRDSEventsFake) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	return &rds.DescribeEventsOutput{Events: f.events}, nil
}

func ridRDSEvent(at time.Time, message string, categories ...string) rdstypes.Event {
	return rdstypes.Event{
		Date:             aws.Time(at),
		Message:          aws.String(message),
		EventCategories:  categories,
		SourceIdentifier: aws.String("acme-orders-db"),
		SourceType:       rdstypes.SourceTypeDbInstance,
		SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:acme-orders-db"),
	}
}

func ridRDSEventRows(t *testing.T, events ...rdstypes.Event) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchRDSEvents(context.Background(), &ridRDSEventsFake{events: events}, "acme-orders-db", "")
	if err != nil {
		t.Fatalf("FetchRDSEvents: %v", err)
	}
	if len(out.Resources) != len(events) {
		t.Fatalf("fetched %d rows, want %d", len(out.Resources), len(events))
	}
	return out.Resources
}

// TestRowIdentity_RDSEvents_EventsInOneMinuteAreDistinct: a reboot writes
// several events inside one minute, two of them at the same second.
func TestRowIdentity_RDSEvents_EventsInOneMinuteAreDistinct(t *testing.T) {
	restartAt := time.Date(2026, 9, 18, 3, 4, 22, 0, time.UTC)
	rows := ridRDSEventRows(t,
		ridRDSEvent(time.Date(2026, 9, 18, 3, 4, 5, 0, time.UTC), "DB instance shutdown", "availability"),
		ridRDSEvent(restartAt, "DB instance restarted", "availability"),
		ridRDSEvent(restartAt, "The database instance has recovered.", "recovery"),
	)
	ridAssertDistinctIDs(t, rows)
}

// TestRowIdentity_RDSEvents_IDSurvivesTheWindowMoving: DescribeEvents answers
// a trailing time window oldest first, so a later call drops the oldest event
// off the front. The events still shown keep their IDs.
func TestRowIdentity_RDSEvents_IDSurvivesTheWindowMoving(t *testing.T) {
	oldest := ridRDSEvent(time.Date(2026, 9, 11, 3, 0, 40, 0, time.UTC), "Backing up DB instance", "backup")
	restarted := ridRDSEvent(time.Date(2026, 9, 18, 3, 4, 22, 0, time.UTC), "DB instance restarted", "availability")
	newest := ridRDSEvent(time.Date(2026, 9, 18, 4, 0, 1, 0, time.UTC), "Finished DB Instance backup", "backup")

	first := ridRDSEventRows(t, oldest, restarted)
	second := ridRDSEventRows(t, restarted, newest)
	if first[1].ID != second[0].ID {
		t.Errorf("the same RDS event has ID %q in one response and %q in the next", first[1].ID, second[0].ID)
	}
}

// ---------------------------------------------------------------------------
// CloudWatch log events and CodeBuild log events (both GetLogEvents)
// ---------------------------------------------------------------------------

type ridGetLogEventsFake struct {
	events []cwltypes.OutputLogEvent
}

func (f *ridGetLogEventsFake) GetLogEvents(_ context.Context, _ *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	return &cloudwatchlogs.GetLogEventsOutput{
		Events:            f.events,
		NextForwardToken:  aws.String("f/38472615273641092837465019283746501928374650192837"),
		NextBackwardToken: aws.String("b/38472615273641092837465019283746501928374650192800"),
	}, nil
}

func ridLogEvent(ms int64, message string) cwltypes.OutputLogEvent {
	return cwltypes.OutputLogEvent{
		Timestamp:     aws.Int64(ms),
		IngestionTime: aws.Int64(ms + 212),
		Message:       aws.String(message),
	}
}

// ridLogFetchers are the two fetchers that read one log stream through
// GetLogEvents; both key their rows the same way.
var ridLogFetchers = []struct {
	name  string
	fetch func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error)
}{
	{"log_events", func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error) {
		return awsclient.FetchLogEvents(context.Background(), api, "/ecs/acme-api", "api/api/9f1c2b7e4d3a4f0e", "")
	}},
	{"cb_build_logs", func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error) {
		return awsclient.FetchCBBuildLogs(context.Background(), api, "/aws/codebuild/acme-api-build", "5b3c9d2e-7f41-4a8b-9c0d-1e2f3a4b5c6d", "")
	}},
}

func ridLogRows(t *testing.T, fetch func(awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error), events ...cwltypes.OutputLogEvent) []resource.Resource {
	t.Helper()
	out, err := fetch(&ridGetLogEventsFake{events: events})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out.Resources) != len(events) {
		t.Fatalf("fetched %d rows, want %d", len(out.Resources), len(events))
	}
	return out.Resources
}

// TestRowIdentity_LogEvents_IDSurvivesTheWindowMoving: GetLogEvents with
// StartFromHead=false answers the newest window of a stream, oldest first, so
// once a new line is written the next response drops the oldest line off the
// front. The lines still shown keep their IDs.
func TestRowIdentity_LogEvents_IDSurvivesTheWindowMoving(t *testing.T) {
	const base = int64(1789726507123) // 2026-09-18 10:15:07.123 UTC
	for _, f := range ridLogFetchers {
		t.Run(f.name, func(t *testing.T) {
			accepted := ridLogEvent(base-5_000, "INFO accepted order=ord-7731")
			errLine := ridLogEvent(base, "ERROR payment gateway timeout after 30000ms order=ord-7731")
			retryLine := ridLogEvent(base+41, "WARN retrying payment for order=ord-7731 attempt=2")

			first := ridLogRows(t, f.fetch, accepted, errLine)
			second := ridLogRows(t, f.fetch, errLine, retryLine)
			if first[1].ID != second[0].ID {
				t.Errorf("the ERROR line has ID %q in one response and %q in the next", first[1].ID, second[0].ID)
			}
		})
	}
}

// TestRowIdentity_LogEvents_SameMillisecondLinesAreDistinct: two different
// lines written in the same millisecond are two rows.
func TestRowIdentity_LogEvents_SameMillisecondLinesAreDistinct(t *testing.T) {
	const at = int64(1789726507123)
	for _, f := range ridLogFetchers {
		t.Run(f.name, func(t *testing.T) {
			rows := ridLogRows(t, f.fetch,
				ridLogEvent(at, "START RequestId: 1b9e0c4f-2d3a-4e5b-8c7d-6f5e4d3c2b1a Version: $LATEST"),
				ridLogEvent(at, "INFO cold start init 412ms"),
			)
			ridAssertDistinctIDs(t, rows)
		})
	}
}
