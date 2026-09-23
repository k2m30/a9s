// aws_log_views_newest_first_test.go — the ECS service log view and the
// Lambda invocation list open on the newest events of their log group.
//
// FilterLogEvents answers oldest-first: a read that takes the first pages of a
// window and stops at a cap holds the oldest events of that window, however
// they are ordered afterwards. A view that promises the latest events must
// show the newest ones — and the oldest line it shows must be newer than every
// line it leaves out.
package unit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fakeLogGroup answers FilterLogEvents for one log group the way CloudWatch
// Logs does: events inside [StartTime, EndTime] (both inclusive) that match
// FilterPattern and the stream selectors, oldest first, at most Limit
// (default and maximum 10000) per page, with an opaque NextToken that is only
// valid for the same query. With scanOnly set it models a group too large to
// search within one call: every page is empty and carries a NextToken. With
// perCall set it models a search that returns only a few events per call
// (CloudWatch Logs may answer fewer than Limit and a NextToken).
type fakeLogGroup struct {
	name     string
	events   []cwlogstypes.FilteredLogEvent
	scanOnly bool
	perCall  int

	calls      int
	tokens     map[string]fakeLogCursor
	seenTokens []string
}

type fakeLogCursor struct {
	query  string
	offset int
}

func (f *fakeLogGroup) FilterLogEvents(_ context.Context, in *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	f.calls++
	group := aws.ToString(in.LogGroupName)
	if group == "" {
		group = aws.ToString(in.LogGroupIdentifier)
	}
	if group != f.name {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "The specified log group does not exist."}
	}
	limit := 10000
	if in.Limit != nil {
		if *in.Limit < 1 || *in.Limit > 10000 {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "Limit must be between 1 and 10000"}
		}
		limit = int(*in.Limit)
	}
	if f.perCall > 0 {
		limit = min(limit, f.perCall)
	}
	if len(in.LogStreamNames) > 0 && in.LogStreamNamePrefix != nil {
		return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "logStreamNames and logStreamNamePrefix are mutually exclusive"}
	}
	pattern := aws.ToString(in.FilterPattern)
	if strings.ContainsAny(pattern, "{[%?") {
		return nil, fmt.Errorf("fake FilterLogEvents: filter pattern %q is not modelled", pattern)
	}
	query := fmt.Sprintf("%d|%d|%s|%v|%s", aws.ToInt64(in.StartTime), aws.ToInt64(in.EndTime), pattern, in.LogStreamNames, aws.ToString(in.LogStreamNamePrefix))

	offset := 0
	if in.NextToken != nil {
		f.seenTokens = append(f.seenTokens, *in.NextToken)
		cur, ok := f.tokens[*in.NextToken]
		if !ok || cur.query != query {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "The specified nextToken is invalid."}
		}
		offset = cur.offset
	}
	if f.tokens == nil {
		f.tokens = map[string]fakeLogCursor{}
	}
	if f.scanOnly {
		tok := fmt.Sprintf("scan-%d", f.calls)
		f.tokens[tok] = fakeLogCursor{query: query}
		return &cloudwatchlogs.FilterLogEventsOutput{NextToken: aws.String(tok)}, nil
	}

	var matched []cwlogstypes.FilteredLogEvent
	for _, e := range f.events {
		ts := aws.ToInt64(e.Timestamp)
		if in.StartTime != nil && ts < *in.StartTime {
			continue
		}
		if in.EndTime != nil && ts > *in.EndTime {
			continue
		}
		if len(in.LogStreamNames) > 0 && !slices.Contains(in.LogStreamNames, aws.ToString(e.LogStreamName)) {
			continue
		}
		if !strings.HasPrefix(aws.ToString(e.LogStreamName), aws.ToString(in.LogStreamNamePrefix)) {
			continue
		}
		if !logPatternMatches(pattern, aws.ToString(e.Message)) {
			continue
		}
		matched = append(matched, e)
	}
	end := min(offset+limit, len(matched))
	out := &cloudwatchlogs.FilterLogEventsOutput{Events: matched[offset:end]}
	if end < len(matched) {
		tok := fmt.Sprintf("tok-%d-%d", f.calls, end)
		f.tokens[tok] = fakeLogCursor{query: query, offset: end}
		out.NextToken = aws.String(tok)
	}
	return out, nil
}

// logPatternMatches implements CloudWatch Logs term matching: a quoted
// pattern matches as one phrase, otherwise every space-separated term must
// appear.
func logPatternMatches(pattern, msg string) bool {
	if pattern == "" {
		return true
	}
	if strings.HasPrefix(pattern, `"`) && strings.HasSuffix(pattern, `"`) && len(pattern) > 1 {
		return strings.Contains(msg, pattern[1:len(pattern)-1])
	}
	for _, term := range strings.Fields(pattern) {
		if !strings.Contains(msg, term) {
			return false
		}
	}
	return true
}

func logEventAt(i int, ts time.Time, stream, msg string) cwlogstypes.FilteredLogEvent {
	return cwlogstypes.FilteredLogEvent{
		EventId:       aws.String(fmt.Sprintf("3751644482337628651642378611738463583744640449604%07d", i)),
		Timestamp:     aws.Int64(ts.UnixMilli()),
		IngestionTime: aws.Int64(ts.Add(900 * time.Millisecond).UnixMilli()),
		LogStreamName: aws.String(stream),
		Message:       aws.String(msg),
	}
}

// newestFirstIDs returns every event's id ordered newest first.
func newestFirstIDs(events []cwlogstypes.FilteredLogEvent, id func(cwlogstypes.FilteredLogEvent) string) []string {
	sorted := slices.Clone(events)
	slices.SortStableFunc(sorted, func(a, b cwlogstypes.FilteredLogEvent) int {
		return int(aws.ToInt64(b.Timestamp) - aws.ToInt64(a.Timestamp))
	})
	ids := make([]string, 0, len(sorted))
	for _, e := range sorted {
		ids = append(ids, id(e))
	}
	return ids
}

// assertNewestPrefix pins that the shown rows are exactly the newest
// len(shown) events, newest first: the oldest one shown is newer than every
// event left out.
func assertNewestPrefix(t *testing.T, shown, newestFirst []string) {
	t.Helper()
	if len(shown) == 0 {
		t.Fatalf("view shows no events, want the newest of %d", len(newestFirst))
	}
	if len(shown) > len(newestFirst) {
		t.Fatalf("view shows %d events, the group holds %d", len(shown), len(newestFirst))
	}
	want := newestFirst[:len(shown)]
	if !slices.Equal(shown, want) {
		t.Errorf("view shows %d events starting %v, want the newest %d starting %v",
			len(shown), shown[:min(3, len(shown))], len(want), want[:min(3, len(want))])
	}
}

// walkLoadMore opens a view and presses Load More until the view reports
// nothing older is left, returning the ids of each page in order. A
// continuation returned twice would make Load More repeat itself forever.
func walkLoadMore(t *testing.T, fetch func(token string) (resource.FetchResult, error)) [][]string {
	t.Helper()
	var pages [][]string
	token := ""
	seen := map[string]bool{}
	for range 60 {
		res, err := fetch(token)
		if err != nil {
			t.Fatalf("page %d: %v", len(pages)+1, err)
		}
		pages = append(pages, resourceIDs(res.Resources))
		if res.Pagination == nil || !res.Pagination.IsTruncated {
			return pages
		}
		next := res.Pagination.NextToken
		if next == "" {
			t.Fatalf("page %d reports more rows but carries no token to load them", len(pages))
		}
		if seen[next] || next == token {
			t.Fatalf("page %d returned continuation %q again: Load More makes no progress", len(pages), next)
		}
		seen[next] = true
		token = next
	}
	t.Fatalf("Load More did not reach the end in 60 pages")
	return nil
}

const ecsWebGroup = "/ecs/acme-web"

func ecsWebTaskDefinition() *mockECSDescribeTaskDefinitionClient {
	return &mockECSDescribeTaskDefinitionClient{output: &ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &ecstypes.TaskDefinition{
			Family:   aws.String("acme-web"),
			Revision: 42,
			ContainerDefinitions: []ecstypes.ContainerDefinition{{
				Name:  aws.String("web"),
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-web:2026.09.1"),
				LogConfiguration: &ecstypes.LogConfiguration{
					LogDriver: ecstypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":         ecsWebGroup,
						"awslogs-region":        "us-east-1",
						"awslogs-stream-prefix": "ecs",
					},
				},
			}},
		},
	}}
}

// ecsWebEvents is n access-log lines, one every step, the newest a minute
// ago, split across the service's two tasks' streams.
func ecsWebEvents(now time.Time, n int, step time.Duration) []cwlogstypes.FilteredLogEvent {
	streams := []string{"ecs/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69", "ecs/web/9e8d7c6b5a4f4e3d2c1b0a9f8e7d6c5b"}
	events := make([]cwlogstypes.FilteredLogEvent, n)
	oldest := now.Add(-time.Minute - time.Duration(n-1)*step)
	for i := range events {
		msg := fmt.Sprintf(`10.0.12.%d - - "GET /api/orders/%d HTTP/1.1" 200 512`, 10+i%200, 100000+i)
		if i%97 == 0 {
			msg = fmt.Sprintf("ERROR order %d: upstream timeout after 30s", 100000+i)
		}
		events[i] = logEventAt(i, oldest.Add(time.Duration(i)*step), streams[i%2], msg)
	}
	return events
}

func fetchEcsWebLogs(t *testing.T, group *fakeLogGroup) resource.FetchResult {
	t.Helper()
	res, err := awsclient.FetchEcsSvcLogs(context.Background(), ecsWebTaskDefinition(), group,
		"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod",
		"acme-web",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/acme-web:42",
		"")
	if err != nil {
		t.Fatalf("FetchEcsSvcLogs: %v", err)
	}
	return res
}

func ecsEventID(e cwlogstypes.FilteredLogEvent) string { return aws.ToString(e.EventId) }

// Five weeks of a busy service: far more lines than the view holds.
func TestEcsSvcLogs_ShowsNewestEventsNewestFirst(t *testing.T) {
	events := ecsWebEvents(time.Now(), 5*7*24*20, 3*time.Minute)
	group := &fakeLogGroup{name: ecsWebGroup, events: events}

	res := fetchEcsWebLogs(t, group)
	shown := resourceIDs(res.Resources)
	if len(shown) >= len(events) {
		t.Fatalf("view shows all %d events; the scenario needs more events than the view holds", len(events))
	}
	assertNewestPrefix(t, shown, newestFirstIDs(events, ecsEventID))
}

// A group that fits in the view is shown whole, newest first.
func TestEcsSvcLogs_SmallGroupShownWholeNewestFirst(t *testing.T) {
	events := ecsWebEvents(time.Now(), 30, 11*time.Hour)
	group := &fakeLogGroup{name: ecsWebGroup, events: events}

	res := fetchEcsWebLogs(t, group)
	want := newestFirstIDs(events, ecsEventID)
	if got := resourceIDs(res.Resources); !slices.Equal(got, want) {
		t.Errorf("view shows %d events %v…, want all %d newest first %v…", len(got), got[:min(3, len(got))], len(want), want[:3])
	}
}

// When the bounded read runs out of calls before it reaches any event, the
// view is not presented as a complete answer.
func TestEcsSvcLogs_UnreachedNewestIsPartial(t *testing.T) {
	group := &fakeLogGroup{name: ecsWebGroup, events: ecsWebEvents(time.Now(), 100, time.Minute), scanOnly: true}

	res, err := awsclient.FetchEcsSvcLogs(context.Background(), ecsWebTaskDefinition(), group,
		"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod",
		"acme-web",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/acme-web:42",
		"")
	if err == nil && (res.Pagination == nil || !res.Pagination.IsTruncated) {
		t.Errorf("read stopped after %d calls without reaching an event, but the result (%d rows) is presented as complete",
			group.calls, len(res.Resources))
	}
}

const lambdaOrdersGroup = "/aws/lambda/acme-orders-api"

func lambdaRequestID(i int) string { return fmt.Sprintf("5f3c9a2e-7b1d-4c8e-9a6f-%012d", i) }

// lambdaInvocations writes START, one application line, END and REPORT for
// each invocation, one every step, the newest ending at last.
func lambdaInvocations(first int, last time.Time, n int, step time.Duration) []cwlogstypes.FilteredLogEvent {
	var events []cwlogstypes.FilteredLogEvent
	start := last.Add(-time.Duration(n-1) * step)
	for k := range n {
		i := first + k
		at := start.Add(time.Duration(k) * step)
		stream := at.UTC().Format("2006/01/02") + "/[$LATEST]8c1f4e2a9b7d4c3e8f6a1b2c3d4e5f60"
		rid := lambdaRequestID(i)
		events = append(events,
			logEventAt(4*i, at, stream, "START RequestId: "+rid+" Version: $LATEST\n"),
			logEventAt(4*i+1, at.Add(3*time.Millisecond), stream, fmt.Sprintf("INFO order %d accepted\n", 500000+i)),
			logEventAt(4*i+2, at.Add(80*time.Millisecond), stream, "END RequestId: "+rid+"\n"),
			logEventAt(4*i+3, at.Add(81*time.Millisecond), stream, fmt.Sprintf(
				"REPORT RequestId: %s\tDuration: %d.%02d ms\tBilled Duration: %d ms\tMemory Size: 512 MB\tMax Memory Used: %d MB\t\n",
				rid, 70+i%40, i%100, 71+i%40, 80+i%30)),
		)
	}
	return events
}

func reportRequestID(e cwlogstypes.FilteredLogEvent) string {
	msg := aws.ToString(e.Message)
	if !strings.HasPrefix(msg, "REPORT RequestId: ") {
		return ""
	}
	return strings.Fields(msg)[2]
}

// newestReportIDs lists the request ids of REPORT lines at or after since,
// newest first.
func newestReportIDs(events []cwlogstypes.FilteredLogEvent, since time.Time) []string {
	var reports []cwlogstypes.FilteredLogEvent
	for _, e := range events {
		if reportRequestID(e) != "" && aws.ToInt64(e.Timestamp) >= since.UnixMilli() {
			reports = append(reports, e)
		}
	}
	return newestFirstIDs(reports, reportRequestID)
}

// 300 invocations in the last 24 hours (and older ones before that): the list
// holds the newest 50 REPORT lines, newest first.
func TestLambdaInvocations_ShowsNewest50NewestFirst(t *testing.T) {
	now := time.Now()
	events := append(
		lambdaInvocations(0, now.Add(-50*time.Hour), 120, 12*time.Minute),
		lambdaInvocations(1000, now.Add(-2*time.Minute), 300, 270*time.Second)...,
	)
	group := &fakeLogGroup{name: lambdaOrdersGroup, events: events}

	res, err := awsclient.FetchLambdaInvocations(context.Background(), group, "acme-orders-api", lambdaOrdersGroup, "")
	if err != nil {
		t.Fatalf("FetchLambdaInvocations: %v", err)
	}
	shown := resourceIDs(res.Resources)
	want := newestReportIDs(events, now.Add(-24*time.Hour))
	if len(shown) != 50 {
		t.Errorf("list shows %d invocations, want 50", len(shown))
	}
	assertNewestPrefix(t, shown, want)
}

// Fewer than 50 invocations in the last 24 hours: all of them, newest first,
// and none from before the window.
func TestLambdaInvocations_FewRecentShownWholeNewestFirst(t *testing.T) {
	now := time.Now()
	events := append(
		lambdaInvocations(0, now.Add(-30*time.Hour), 80, 20*time.Minute),
		lambdaInvocations(1000, now.Add(-5*time.Minute), 12, 90*time.Minute)...,
	)
	group := &fakeLogGroup{name: lambdaOrdersGroup, events: events}

	res, err := awsclient.FetchLambdaInvocations(context.Background(), group, "acme-orders-api", lambdaOrdersGroup, "")
	if err != nil {
		t.Fatalf("FetchLambdaInvocations: %v", err)
	}
	want := newestReportIDs(events, now.Add(-24*time.Hour))
	if got := resourceIDs(res.Resources); !slices.Equal(got, want) {
		t.Errorf("list shows %v, want the 12 invocations of the last 24h newest first %v", got, want)
	}
}

// A burst of lines inside the newest few minutes, answered a few events per
// call, holds more than one read's call budget. Load More still moves on:
// the pages join newest first with no gap or repeat and reach every line.
func TestEcsSvcLogs_BudgetExhaustedWindowStillProgresses(t *testing.T) {
	events := ecsWebEvents(time.Now(), 300, 200*time.Millisecond)
	group := &fakeLogGroup{name: ecsWebGroup, events: events, perCall: 1}

	pages := walkLoadMore(t, func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEcsSvcLogs(context.Background(), ecsWebTaskDefinition(), group,
			"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod",
			"acme-web",
			"arn:aws:ecs:us-east-1:123456789012:task-definition/acme-web:42",
			token)
	})
	var all []string
	for _, p := range pages {
		all = append(all, p...)
	}
	if want := newestFirstIDs(events, ecsEventID); !slices.Equal(all, want) {
		t.Errorf("Load More walk shows %d lines over %d pages, want all %d newest first", len(all), len(pages), len(want))
	}
}

func TestLambdaInvocations_BudgetExhaustedWindowStillProgresses(t *testing.T) {
	now := time.Now()
	events := lambdaInvocations(0, now.Add(-time.Minute), 150, time.Second)
	group := &fakeLogGroup{name: lambdaOrdersGroup, events: events, perCall: 1}

	pages := walkLoadMore(t, func(token string) (resource.FetchResult, error) {
		return awsclient.FetchLambdaInvocations(context.Background(), group, "acme-orders-api", lambdaOrdersGroup, token)
	})
	var all []string
	for _, p := range pages {
		all = append(all, p...)
	}
	if want := newestReportIDs(events, now.Add(-24*time.Hour)); !slices.Equal(all, want) {
		t.Errorf("Load More walk shows %d invocations over %d pages, want all %d newest first", len(all), len(pages), len(want))
	}
}

// awslogs names a stream <awslogs-stream-prefix>/<container-name>/<task-id>.
// Services that share one log group are told apart by that prefix, and a
// service's view holds the streams of every awslogs container in its task
// definition — never another service's, including one whose prefix merely
// starts with this one's.
func TestEcsSvcLogs_ReadsOnlyTheServicesStreams(t *testing.T) {
	const shared = "/ecs/acme-shared"
	now := time.Now()
	var events []cwlogstypes.FilteredLogEvent
	var mine []cwlogstypes.FilteredLogEvent
	streams := []struct {
		name string
		own  bool
	}{
		{"acme-web/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69", true},
		{"acme-web/envoy/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69", true},
		{"acme-worker/worker/9e8d7c6b5a4f4e3d2c1b0a9f8e7d6c5b", false},
		{"acme-web-canary/web/0a1b2c3d4e5f40718293a4b5c6d7e8f9", false},
	}
	for i := range 160 {
		s := streams[i%len(streams)]
		e := logEventAt(i, now.Add(-time.Duration(160-i)*time.Minute), s.name, fmt.Sprintf("INFO %s request %d served", s.name, i))
		events = append(events, e)
		if s.own {
			mine = append(mine, e)
		}
	}
	group := &fakeLogGroup{name: shared, events: events}
	taskDef := &mockECSDescribeTaskDefinitionClient{output: &ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &ecstypes.TaskDefinition{
			Family:   aws.String("acme-web"),
			Revision: 42,
			ContainerDefinitions: []ecstypes.ContainerDefinition{
				{Name: aws.String("xray-daemon"), Image: aws.String("public.ecr.aws/xray/aws-xray-daemon:3.3")},
				{Name: aws.String("web"), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-web:2026.09.1"),
					LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{
						"awslogs-group": shared, "awslogs-region": "us-east-1", "awslogs-stream-prefix": "acme-web"}}},
				{Name: aws.String("envoy"), Image: aws.String("public.ecr.aws/appmesh/aws-appmesh-envoy:v1.29.4.0-prod"),
					LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{
						"awslogs-group": shared, "awslogs-region": "us-east-1", "awslogs-stream-prefix": "acme-web"}}},
			},
		},
	}}

	res, err := awsclient.FetchEcsSvcLogs(context.Background(), taskDef, group,
		"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod",
		"acme-web",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/acme-web:42",
		"")
	if err != nil {
		t.Fatalf("FetchEcsSvcLogs: %v", err)
	}
	want := newestFirstIDs(mine, ecsEventID)
	if got := resourceIDs(res.Resources); !slices.Equal(got, want) {
		var foreign []string
		for _, r := range res.Resources {
			if !strings.HasPrefix(r.Fields["log_stream"], "acme-web/") {
				foreign = append(foreign, r.Fields["log_stream"])
			}
		}
		t.Errorf("view shows %d lines, want the service's %d newest first; streams of other services shown: %d %v",
			len(got), len(want), len(foreign), foreign[:min(3, len(foreign))])
	}
}
