// t565_log_views_edges_test.go — the edges of the newest-first log views:
// the console link of a line read in another Region, a millisecond busier
// than one read's call budget, and identical lines on either side of a
// GetLogEvents page boundary.
package unit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/resource"
)

func taskDefinitionLoggingTo(options map[string]string) *mockECSDescribeTaskDefinitionClient {
	return &mockECSDescribeTaskDefinitionClient{output: &ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &ecstypes.TaskDefinition{
			Family: aws.String("acme-web"),
			ContainerDefinitions: []ecstypes.ContainerDefinition{{
				Name:             aws.String("web"),
				LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: options},
			}},
		},
	}}
}

// A service line lives in the log group of its awslogs-region, and its
// console link opens that Region's console, not the session's.
func TestEcsSvcLogs_ConsoleLinkOpensTheAwslogsRegion(t *testing.T) {
	events := ecsWebEvents(time.Now(), 5, time.Minute)
	for i := range events {
		events[i].LogStreamName = aws.String("ecs/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69")
	}
	group := &fakeLogGroup{name: ecsWebGroup, events: events}
	td := taskDefinitionLoggingTo(map[string]string{"awslogs-group": ecsWebGroup, "awslogs-region": "eu-west-1", "awslogs-stream-prefix": "ecs"})

	res, err := awsclient.FetchEcsSvcLogs(context.Background(), td, group, "acme-prod", "acme-web", "acme-web:42", "")
	if err != nil {
		t.Fatalf("FetchEcsSvcLogs: %v", err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("view shows no lines")
	}
	ctd := resource.GetChildType("ecs_svc_logs")
	u, ok := consolelink.Resolve(*ctd, res.Resources[0], "us-east-1", "123456789012")
	if !ok || !strings.Contains(u, "region=eu-west-1") || strings.Contains(u, "us-east-1") {
		t.Errorf("console link %q (resolved %v), want the eu-west-1 console", u, ok)
	}
}

// 150 lines share one millisecond and the group answers one event per call,
// so that millisecond alone needs more calls than one read makes. The walk
// still shows every line exactly once, newest first.
func TestEcsSvcLogs_MillisecondBeyondTheBudgetIsReachedByLoadMore(t *testing.T) {
	now := time.Now()
	events := ecsWebEvents(now, 180, time.Second)
	busy := now.Add(-2 * time.Minute).UnixMilli()
	for i := 10; i < 160; i++ {
		events[i].Timestamp = aws.Int64(busy)
	}
	group := &fakeLogGroup{name: ecsWebGroup, events: events, perCall: 1}

	pages := walkLoadMore(t, func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEcsSvcLogs(context.Background(), ecsWebTaskDefinition(), group,
			"acme-prod", "acme-web", "acme-web:42", token)
	})
	var all []string
	for _, p := range pages {
		all = append(all, p...)
	}
	got, want := slices.Clone(all), newestFirstIDs(events, ecsEventID)
	slices.Sort(got)
	sortedWant := slices.Clone(want)
	slices.Sort(sortedWant)
	if !slices.Equal(got, sortedWant) {
		t.Errorf("walk shows %d lines over %d pages, want all %d exactly once", len(all), len(pages), len(want))
	}
}

// Identical lines in one millisecond on either side of a GetLogEvents page
// boundary are two events, and stay two rows once Load More has read both.
func TestLogEvents_TwinLinesAcrossAPageBoundaryStayTwoRows(t *testing.T) {
	const group, stream = "/ecs/acme-web", "acme-web/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69"
	events := streamEvents(12000, func(i int) string { return fmt.Sprintf("line %06d", i) })
	// The newest page holds 4262 events of this size; the twins sit at the
	// last event of the page before it and the first event of the newest.
	boundary := len(events) - 4262
	twin := cwlogstypes.OutputLogEvent{
		Timestamp:     events[boundary].Timestamp,
		IngestionTime: events[boundary].IngestionTime,
		Message:       aws.String(strings.Repeat("retrying artifact upload ", 9)[:220]),
	}
	events[boundary-1], events[boundary] = twin, twin
	api := &fakeLogStream{group: group, stream: stream, events: events}

	for name, fetch := range map[string]func(token string) (resource.FetchResult, error){
		"log_events": func(token string) (resource.FetchResult, error) {
			return awsclient.FetchLogEvents(context.Background(), api, group, stream, token)
		},
		"cb_build_logs": func(token string) (resource.FetchResult, error) {
			return awsclient.FetchCBBuildLogs(context.Background(), api, group, stream, token)
		},
	} {
		ids := map[string]int{}
		token := ""
		for range 10 {
			res, err := fetch(token)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, r := range res.Resources {
				ids[r.ID]++
			}
			if res.Pagination == nil || !res.Pagination.IsTruncated {
				break
			}
			token = res.Pagination.NextToken
		}
		if len(ids) != len(events) {
			t.Errorf("%s: %d distinct row IDs over %d events", name, len(ids), len(events))
		}
		for id, n := range ids {
			if n > 1 {
				t.Errorf("%s: row ID %s is carried by %d rows", name, id, n)
			}
		}
	}
}
