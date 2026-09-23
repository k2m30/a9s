// aws_log_stream_pages_test.go — the log-stream event view and the CodeBuild
// build log view over a stream longer than one GetLogEvents page.
//
// GetLogEvents without a token (startFromHead=false) answers the newest page
// of a stream, up to 10,000 events or 1 MB, in chronological order. Older
// pages are reached through nextBackwardToken, and AWS signals the start of
// the stream by returning the token it was sent. A view that opens on the
// newest page holds only part of the stream: it is marked truncated, and Load
// More walks older until the stream's start.
package unit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fakeLogStream answers GetLogEvents for one stream the way CloudWatch Logs
// does: a page is at most Limit events (default and maximum 10,000) and at
// most 1,048,576 bytes, counting each event as its UTF-8 message plus 26
// bytes; events inside a page are oldest first; nextBackwardToken reaches
// the page before, nextForwardToken the page after, and at either end of the
// stream the token sent is the token returned. A forward token is valid only
// with startFromHead=true.
type fakeLogStream struct {
	group, stream string
	events        []cwlogstypes.OutputLogEvent

	calls int
}

const getLogEventsMaxBytes = 1048576

func (f *fakeLogStream) GetLogEvents(_ context.Context, in *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	f.calls++
	if aws.ToString(in.LogGroupName) != f.group || aws.ToString(in.LogStreamName) != f.stream {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "The specified log stream does not exist."}
	}
	limit := 10000
	if in.Limit != nil {
		if *in.Limit < 1 || *in.Limit > 10000 {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "Limit must be between 1 and 10000"}
		}
		limit = int(*in.Limit)
	}
	fromHead := aws.ToBool(in.StartFromHead)
	n := len(f.events)

	backward, pos := !fromHead, n
	if fromHead {
		pos = 0
	}
	if in.NextToken != nil {
		tok := *in.NextToken
		dir, idx, ok := strings.Cut(tok, "/")
		i, err := strconv.Atoi(idx)
		if !ok || err != nil || i < 0 || i > n || (dir != "b" && dir != "f") {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "The specified nextToken is invalid."}
		}
		if dir == "f" && !fromHead {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "startFromHead must be true when using a nextForwardToken"}
		}
		backward, pos = dir == "b", i
	}

	var lo, hi int
	if backward {
		hi, lo = pos, pos
		size := 0
		for lo > 0 && hi-lo < limit {
			s := len(aws.ToString(f.events[lo-1].Message)) + 26
			if size+s > getLogEventsMaxBytes {
				break
			}
			size += s
			lo--
		}
	} else {
		lo, hi = pos, pos
		size := 0
		for hi < n && hi-lo < limit {
			s := len(aws.ToString(f.events[hi].Message)) + 26
			if size+s > getLogEventsMaxBytes {
				break
			}
			size += s
			hi++
		}
	}
	return &cloudwatchlogs.GetLogEventsOutput{
		Events:            f.events[lo:hi],
		NextBackwardToken: aws.String("b/" + strconv.Itoa(lo)),
		NextForwardToken:  aws.String("f/" + strconv.Itoa(hi)),
	}, nil
}

// streamEvents is n lines of about 220 bytes, one a second, the newest a
// minute ago: n=12000 spans three GetLogEvents pages.
func streamEvents(n int, line func(i int) string) []cwlogstypes.OutputLogEvent {
	events := make([]cwlogstypes.OutputLogEvent, n)
	start := time.Now().Add(-time.Minute - time.Duration(n)*time.Second)
	for i := range events {
		ts := start.Add(time.Duration(i) * time.Second)
		msg := line(i)
		msg += strings.Repeat(" ", max(0, 220-len(msg)))
		events[i] = cwlogstypes.OutputLogEvent{
			Timestamp:     aws.Int64(ts.UnixMilli()),
			IngestionTime: aws.Int64(ts.Add(700 * time.Millisecond).UnixMilli()),
			Message:       aws.String(msg),
		}
	}
	return events
}

// streamRowKey identifies a row by the event it carries: GetLogEvents events
// have no id, and every fixture line is unique.
func streamRowKey(t *testing.T, r resource.Resource) string {
	t.Helper()
	raw, ok := r.RawStruct.(cwlogstypes.OutputLogEvent)
	if !ok {
		t.Fatalf("row %q RawStruct is %T, want cwlogstypes.OutputLogEvent", r.ID, r.RawStruct)
	}
	return aws.ToString(raw.Message)
}

// assertStreamWalk opens the view, then presses Load More until it reports
// the stream's start, and pins the contract of a view over a long stream:
// the first page is the newest events and is marked truncated with a way to
// load more; each later page is older than everything already shown; no page
// claims an exact total while more remain; the walk ends, and together the
// pages hold every event of the stream exactly once.
func assertStreamWalk(t *testing.T, events []cwlogstypes.OutputLogEvent, fetch func(token string) (resource.FetchResult, error)) {
	t.Helper()
	index := make(map[string]int, len(events))
	for i, e := range events {
		index[aws.ToString(e.Message)] = i
	}
	seen := map[int]bool{}
	oldestShown := len(events)
	token := ""
	tokens := map[string]bool{}
	for page := 1; ; page++ {
		if page > 20 {
			t.Fatalf("Load More did not reach the stream's start in 20 pages (%d of %d events shown)", len(seen), len(events))
		}
		res, err := fetch(token)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		pageMin, pageMax := len(events), -1
		for _, r := range res.Resources {
			i, ok := index[streamRowKey(t, r)]
			if !ok {
				t.Fatalf("page %d shows a line the stream does not hold: %q", page, streamRowKey(t, r))
			}
			if seen[i] {
				t.Errorf("page %d repeats event %d", page, i)
			}
			seen[i] = true
			pageMin, pageMax = min(pageMin, i), max(pageMax, i)
		}
		if page == 1 {
			if len(res.Resources) == 0 || len(res.Resources) >= len(events) {
				t.Fatalf("first page shows %d of %d events; the scenario needs a stream longer than one page", len(res.Resources), len(events))
			}
			if pageMin != len(events)-len(res.Resources) || pageMax != len(events)-1 {
				t.Errorf("first page holds events %d..%d, want the newest %d (%d..%d)", pageMin, pageMax, len(res.Resources), len(events)-len(res.Resources), len(events)-1)
			}
		} else if pageMax >= oldestShown {
			t.Errorf("page %d holds event %d, not older than event %d already shown", page, pageMax, oldestShown)
		}
		oldestShown = min(oldestShown, pageMin)

		p := res.Pagination
		if len(seen) < len(events) {
			if p == nil || !p.IsTruncated || p.NextToken == "" {
				t.Fatalf("page %d: %d of %d events shown but the view offers no Load More (%+v)", page, len(seen), len(events), p)
			}
			if p.TotalHint >= 0 && p.TotalHint <= len(seen) {
				t.Errorf("page %d claims TotalHint %d while %d of %d events are shown", page, p.TotalHint, len(seen), len(events))
			}
		}
		if p == nil || !p.IsTruncated {
			break
		}
		if tokens[p.NextToken] {
			t.Fatalf("page %d returned continuation %q a second time", page, p.NextToken)
		}
		tokens[p.NextToken] = true
		token = p.NextToken
	}
	if len(seen) != len(events) {
		t.Errorf("walk ended with %d of %d events shown", len(seen), len(events))
	}
}

func TestLogEvents_LongStreamOpensOnNewestAndWalksOlder(t *testing.T) {
	const group, stream = "/ecs/acme-web", "acme-web/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69"
	events := streamEvents(12000, func(i int) string {
		return fmt.Sprintf(`10.0.12.%d - - [line %06d] "GET /api/orders/%d HTTP/1.1" 200 512 "-" "ELB-HealthChecker/2.0"`, 10+i%200, i, 100000+i)
	})
	api := &fakeLogStream{group: group, stream: stream, events: events}

	assertStreamWalk(t, events, func(token string) (resource.FetchResult, error) {
		return awsclient.FetchLogEvents(context.Background(), api, group, stream, token)
	})
}

func TestCBBuildLogs_LongBuildLogOpensOnNewestAndWalksOlder(t *testing.T) {
	const group, stream = "/aws/codebuild/acme-api-build", "7d3e9f21-4b6a-4c8d-9e0f-1a2b3c4d5e6f"
	events := streamEvents(12000, func(i int) string {
		return fmt.Sprintf("[Container] 2026/09/23 10:%02d:%02d.%03d line %06d: npm run test -- --shard=%d/8 PASS src/orders/order-%d.spec.ts", (i/60)%60, i%60, i%1000, i, i%8+1, i)
	})
	api := &fakeLogStream{group: group, stream: stream, events: events}

	assertStreamWalk(t, events, func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCBBuildLogs(context.Background(), api, group, stream, token)
	})
}

// A stream that fits in one page is shown whole and is not offered Load More.
func TestLogEvents_ShortStreamIsWhole(t *testing.T) {
	const group, stream = "/ecs/acme-web", "acme-web/web/4f1c2b7a9d8e4c6b8a1f0e2d3c4b5a69"
	events := streamEvents(40, func(i int) string { return fmt.Sprintf("INFO line %06d request served", i) })
	api := &fakeLogStream{group: group, stream: stream, events: events}

	for name, fetch := range map[string]func() (resource.FetchResult, error){
		"log_events": func() (resource.FetchResult, error) {
			return awsclient.FetchLogEvents(context.Background(), api, group, stream, "")
		},
		"cb_build_logs": func() (resource.FetchResult, error) {
			return awsclient.FetchCBBuildLogs(context.Background(), api, group, stream, "")
		},
	} {
		res, err := fetch()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(res.Resources) != 40 {
			t.Errorf("%s: shows %d of 40 events", name, len(res.Resources))
		}
		if res.Pagination != nil && res.Pagination.IsTruncated {
			t.Errorf("%s: a stream shown whole offers Load More (%+v)", name, res.Pagination)
		}
	}
}
