// aws_ecs_svc_logs_region_test.go — the ECS service log view reads the log
// group in the Region the task definition's awslogs-region names.
//
// The awslogs driver writes to the log group in awslogs-region, which need
// not be the Region the service runs in. The group exists only there: a read
// in the session's Region finds no such group.
package unit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type svcLogEvent struct {
	EventID       string `json:"eventId"`
	Timestamp     int64  `json:"timestamp"`
	IngestionTime int64  `json:"ingestionTime"`
	LogStreamName string `json:"logStreamName"`
	Message       string `json:"message"`
}

// svcLogsWorld answers ECS DescribeTaskDefinition in the session's Region and
// CloudWatch Logs FilterLogEvents for each log group only in the Region it
// lives in, over HTTP, so the Region a call went to is its SigV4 scope.
type svcLogsWorld struct {
	taskRegion string
	awslogs    map[string]string // awslogs option → value
	groups     map[string]map[string][]svcLogEvent

	mu       sync.Mutex
	logCalls []string // "<op>@<region>"
}

func (w *svcLogsWorld) RoundTrip(req *http.Request) (*http.Response, error) {
	region, service := rwScope(req.Header.Get("Authorization"))
	_, op, _ := strings.Cut(req.Header.Get("X-Amz-Target"), ".")
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	}
	switch {
	case service == "ecs" && op == "DescribeTaskDefinition":
		if region != w.taskRegion {
			return rwJSONError(400, "ClientException", "Unable to describe task definition."), nil
		}
		return rwJSON(map[string]any{"taskDefinition": map[string]any{
			"taskDefinitionArn": "arn:aws:ecs:" + w.taskRegion + ":123456789012:task-definition/acme-web:42",
			"family":            "acme-web",
			"revision":          42,
			"containerDefinitions": []any{map[string]any{
				"name":             "web",
				"image":            "123456789012.dkr.ecr." + w.taskRegion + ".amazonaws.com/acme-web:2026.09.1",
				"logConfiguration": map[string]any{"logDriver": "awslogs", "options": w.awslogs},
			}},
		}}), nil
	case service == "logs":
		w.mu.Lock()
		w.logCalls = append(w.logCalls, op+"@"+region)
		w.mu.Unlock()
		if op != "FilterLogEvents" {
			return rwJSONError(400, "InvalidOperationException", "logs "+op+" is not modelled"), nil
		}
		var in struct {
			LogGroupName        string `json:"logGroupName"`
			StartTime           *int64 `json:"startTime"`
			EndTime             *int64 `json:"endTime"`
			LogStreamNamePrefix string `json:"logStreamNamePrefix"`
			FilterPattern       string `json:"filterPattern"`
		}
		_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body names no group and is answered as not found
		events, ok := w.groups[region][in.LogGroupName]
		if !ok {
			return rwJSONError(400, "ResourceNotFoundException", "The specified log group does not exist."), nil
		}
		out := []svcLogEvent{}
		for _, e := range events {
			if (in.StartTime != nil && e.Timestamp < *in.StartTime) || (in.EndTime != nil && e.Timestamp > *in.EndTime) {
				continue
			}
			if !strings.HasPrefix(e.LogStreamName, in.LogStreamNamePrefix) || !strings.Contains(e.Message, in.FilterPattern) {
				continue
			}
			out = append(out, e)
		}
		return rwJSON(map[string]any{"events": out}), nil
	}
	return rwJSONError(400, "UnknownOperationException", service+" "+op+" is not modelled"), nil
}

func svcLogLines(region string, n int) []svcLogEvent {
	now := time.Now()
	events := make([]svcLogEvent, n)
	for i := range events {
		ts := now.Add(-time.Duration(n-i) * 7 * time.Minute).UnixMilli()
		events[i] = svcLogEvent{
			EventID:       fmt.Sprintf("3751644482337628651642378611738463583744640449604%07d", i),
			Timestamp:     ts,
			IngestionTime: ts + 800,
			LogStreamName: "acme-web/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69",
			Message:       fmt.Sprintf("INFO %s request %d served", region, i),
		}
	}
	return events
}

func openSvcLogs(t *testing.T, w *svcLogsWorld, sessionRegion string) []resource.Resource {
	t.Helper()
	clients := awsclient.CreateServiceClients(aws.Config{
		Region:           sessionRegion,
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: w},
		RetryMaxAttempts: 1,
	})
	ctd := resource.GetChildType("ecs_svc_logs")
	if ctd == nil || ctd.ChildFetcher == nil {
		t.Fatal("ecs_svc_logs has no child fetcher")
	}
	res, err := ctd.ChildFetcher(context.Background(), clients, resource.ParentContext{
		"cluster":         "arn:aws:ecs:" + sessionRegion + ":123456789012:cluster/acme-prod",
		"service_name":    "acme-web",
		"task_definition": "arn:aws:ecs:" + sessionRegion + ":123456789012:task-definition/acme-web:42",
	}, "")
	if err != nil {
		t.Fatalf("ecs_svc_logs: %v (logs calls %v)", err, w.logCalls)
	}
	return res.Resources
}

func svcLogMessages(rs []resource.Resource) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Fields["message"])
	}
	slices.Sort(out)
	return out
}

func svcLogWant(events []svcLogEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Message)
	}
	slices.Sort(out)
	return out
}

func TestEcsSvcLogs_ReadsTheAwslogsRegion(t *testing.T) {
	t.Run("awslogs-region differs from the session", func(t *testing.T) {
		lines := svcLogLines("eu-west-1", 30)
		w := &svcLogsWorld{
			taskRegion: "us-east-1",
			awslogs:    map[string]string{"awslogs-group": "/ecs/acme-web", "awslogs-region": "eu-west-1", "awslogs-stream-prefix": "acme-web"},
			groups:     map[string]map[string][]svcLogEvent{"eu-west-1": {"/ecs/acme-web": lines}},
		}
		rows := openSvcLogs(t, w, "us-east-1")
		if got, want := svcLogMessages(rows), svcLogWant(lines); !slices.Equal(got, want) {
			t.Errorf("view shows %d lines, want the %d lines of the group in eu-west-1", len(got), len(want))
		}
		for _, c := range w.logCalls {
			if !strings.HasSuffix(c, "@eu-west-1") {
				t.Errorf("CloudWatch Logs call %s went outside awslogs-region eu-west-1 (all calls %v)", c, w.logCalls)
				break
			}
		}
	})

	t.Run("awslogs-region is the session's", func(t *testing.T) {
		lines := svcLogLines("us-east-1", 30)
		w := &svcLogsWorld{
			taskRegion: "us-east-1",
			awslogs:    map[string]string{"awslogs-group": "/ecs/acme-web", "awslogs-region": "us-east-1", "awslogs-stream-prefix": "acme-web"},
			groups:     map[string]map[string][]svcLogEvent{"us-east-1": {"/ecs/acme-web": lines}},
		}
		rows := openSvcLogs(t, w, "us-east-1")
		if got, want := svcLogMessages(rows), svcLogWant(lines); !slices.Equal(got, want) {
			t.Errorf("view shows %d lines, want the %d lines of the group in us-east-1", len(got), len(want))
		}
	})
}
