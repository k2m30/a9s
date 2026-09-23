// aws_lambda_invocations_review_test.go — the invocation list and log over
// failed and SnapStart invocations, a busy function on a shared log group, a
// list read to its end over time, and JSON-format platform lines.
//
// Shapes, as AWS documents them:
//
//   - REPORT line, new style (docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html#runtimes-lifecycle-invoke-with-errors):
//     "... Max Memory Used: 31 MB Init Duration: 80.00 ms Status: error Error Type: Runtime.ExitError";
//     a timeout ends "Status: timeout".
//   - platform.report (docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-report):
//     record.status is success|failure|error|timeout, with record.errorType
//     for failure and error; ReportMetrics carries restoreDurationMs?.
//   - SnapStart (docs.aws.amazon.com/lambda/latest/dg/snapstart-monitoring.html):
//     a new execution environment's REPORT has no Init Duration and carries
//     "Restore Duration" and "Billed Restore Duration"; its cold start is
//     Restore Duration + Duration.
package unit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func invocationsFetcher(world *lambdaLogWorld, pctx map[string]string) func(token string) (resource.FetchResult, error) {
	return func(token string) (resource.FetchResult, error) {
		return resource.GetChildType("lambda_invocations").ChildFetcher(context.Background(), &awsclient.ServiceClients{CloudWatchLogs: world}, pctx, token)
	}
}

func rowByRequestID(t *testing.T, rows []resource.Resource, rid string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.Fields["request_id"] == rid {
			return r
		}
	}
	t.Fatalf("invocation %s not listed (%v)", rid, requestIDsOf(rows))
	return resource.Resource{}
}

// fieldHolding names the field whose value is want, or "".
func fieldHolding(r resource.Resource, want string, skip ...string) string {
	for k, v := range r.Fields {
		if v == want && !slices.Contains(skip, k) {
			return k
		}
	}
	return ""
}

func fieldContaining(r resource.Resource, part string, skip ...string) string {
	for k, v := range r.Fields {
		if strings.Contains(v, part) && !slices.Contains(skip, k) {
			return k
		}
	}
	return ""
}

// ─── row 4: failed invocations ─────────────────────────────────────────────

// An invocation that failed reads as ERROR, names its error type in a field
// of its own, and carries a broken finding of its own beside the timeout
// one; a success stays OK with no finding. Text REPORT lines and JSON
// platform.report records alike.
func TestLambdaInvocations_FailedInvocationIsError(t *testing.T) {
	now := time.Now()
	var textEvents, jsonEvents []cwlogstypes.FilteredLogEvent
	next := 0
	text := lambdaLogWriter{events: &textEvents, next: &next}
	js := lambdaLogWriter{events: &jsonEvents, next: &next}
	reports := map[string]lambdaReport{
		"ok":      {durationMs: "41.07", billedMs: 42, memoryMB: 128, usedMB: 86, status: "success"},
		"oom":     {durationMs: "812.33", billedMs: 813, memoryMB: 128, usedMB: 128, status: "error", errorType: "Runtime.OutOfMemory"},
		"crash":   {durationMs: "133.61", billedMs: 134, memoryMB: 128, usedMB: 31, status: "failure", errorType: "Runtime.ExitError"},
		"timeout": {durationMs: "3003.52", billedMs: 3000, memoryMB: 128, usedMB: 90, status: "timeout"},
	}
	order := []string{"ok", "oom", "crash", "timeout"}
	ids := map[string]string{}
	for i, k := range order {
		at := now.Add(-time.Duration(len(order)-i) * 20 * time.Minute)
		tid := fmt.Sprintf("0b7c1f3e-2a4d-4e6f-8a9b-%012d", 100+i)
		jid := fmt.Sprintf("7e4d2c1b-9f8a-4b3c-8d2e-%012d", 100+i)
		r := reports[k]
		if k != "crash" {
			text.textInvocation(defaultStream(at, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), tid, at, r)
		}
		js.jsonInvocation(defaultStream(at, "f0e9d8c7b6a5948372615049382716a5"), jid, at.Add(time.Second), r)
		ids["text/"+k], ids["json/"+k] = tid, jid
	}
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-orders-api":   {name: "/aws/lambda/acme-orders-api", events: textEvents},
		"/aws/lambda/acme-billing-sync": {name: "/aws/lambda/acme-billing-sync", events: jsonEvents},
	}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-orders-api", "/aws/lambda/acme-orders-api", lambdatypes.LogFormatText),
		lambdaFunction("acme-billing-sync", "/aws/lambda/acme-billing-sync", lambdatypes.LogFormatJson),
	}
	textRows, _ := openInvocations(t, world, fns, "acme-orders-api")
	jsonRows, _ := openInvocations(t, world, fns, "acme-billing-sync")
	rowsOf := map[string][]resource.Resource{"text": textRows, "json": jsonRows}

	timeoutCode := string(awsclient.CodeLambdaInvocationTimeout)
	for _, format := range []string{"text", "json"} {
		for _, k := range order {
			rid, ok := ids[format+"/"+k]
			if !ok || (format == "text" && k == "crash") {
				continue
			}
			row := rowByRequestID(t, rowsOf[format], rid)
			rep := reports[k]
			switch k {
			case "ok":
				if row.Fields["status"] != "OK" || len(row.Findings) != 0 {
					t.Errorf("%s success: status %q findings %v, want OK and none", format, row.Fields["status"], findingCodes(row.Findings))
				}
			case "timeout":
				if got := findingCodes(row.Findings); !slices.Equal(got, []string{timeoutCode}) {
					t.Errorf("%s timeout: findings %v, want [%s]", format, got, timeoutCode)
				}
			default:
				if row.Fields["status"] != "ERROR" {
					t.Errorf("%s %s invocation: status %q, want ERROR", format, rep.status, row.Fields["status"])
				}
				if fieldHolding(row, rep.errorType, "status", "request_id") == "" {
					t.Errorf("%s %s invocation: no field holds the error type %q (fields %v)", format, rep.status, rep.errorType, row.Fields)
				}
				broken := 0
				for _, f := range row.Findings {
					if f.Severity == domain.SevBroken && string(f.Code) != timeoutCode {
						broken++
					}
				}
				if broken != 1 {
					t.Errorf("%s %s invocation: findings %v, want one broken finding that is not the timeout one", format, rep.status, findingCodes(row.Findings))
				}
			}
		}
	}
}

// ─── row 5: a busy function's streams beyond the listing cap ───────────────

// A function writing to a shared log group from more execution environments
// than one bounded stream listing reaches: reading its invocation list to the
// end terminates, reports the incomplete listing as a partial failure beside
// the rows it did read, never offers Load More without a cursor, and shows
// none of the other function's invocations.
func TestLambdaInvocations_StreamListingCapIsPartialNotEndlessLoadMore(t *testing.T) {
	const shared = "/acme/lambda/orders"
	now := time.Now()
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "25.61", billedMs: 26, memoryMB: 128, usedMB: 77, status: "success"}
	own := map[string]bool{}
	for i := range 620 {
		at := now.Add(-time.Duration(620-i) * 15 * time.Second)
		rid := fmt.Sprintf("9a8b7c6d-5e4f-4a3b-9c2d-%012d", i)
		lw.textInvocation(customStream(at, "acme-orders", fmt.Sprintf("%032x", 0x1000+i)), rid, at, ok)
		own[rid] = true
	}
	for i := range 30 {
		at := now.Add(-time.Duration(30-i) * 5 * time.Minute)
		lw.textInvocation(customStream(at, "acme-orders-api", fmt.Sprintf("%032x", 0x9000+i)), fmt.Sprintf("1c2d3e4f-5a6b-4c7d-8e9f-%012d", i), at, ok)
	}
	slices.SortStableFunc(events, func(a, b cwlogstypes.FilteredLogEvent) int {
		return int(*a.Timestamp - *b.Timestamp)
	})
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{shared: {name: shared, events: events}}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-orders", shared, lambdatypes.LogFormatText),
		lambdaFunction("acme-orders-api", shared, lambdatypes.LogFormatText),
	}
	_, pctx := openInvocations(t, world, fns, "acme-orders")
	fetch := invocationsFetcher(world, pctx)

	seen := map[string]bool{}
	tokens := map[string]bool{}
	partial := false
	token := ""
	for page := 1; ; page++ {
		if page > 60 {
			t.Fatalf("Load More did not end in 60 pages (%d invocations shown)", len(seen))
		}
		res, err := fetch(token)
		if err != nil {
			if len(res.Resources) == 0 && res.Pagination == nil {
				t.Fatalf("page %d failed outright: %v", page, err)
			}
			partial = true
		}
		for _, r := range res.Resources {
			rid := r.Fields["request_id"]
			if !own[rid] {
				t.Errorf("page %d lists %s, not an acme-orders invocation", page, rid)
			}
			if seen[rid] {
				t.Errorf("page %d repeats %s", page, rid)
			}
			seen[rid] = true
		}
		p := res.Pagination
		if p != nil && p.IsTruncated && p.NextToken == "" {
			t.Fatalf("page %d is truncated with an empty cursor: Load More has nowhere to go", page)
		}
		if p == nil || !p.IsTruncated {
			break
		}
		if tokens[p.NextToken] || p.NextToken == token {
			t.Fatalf("page %d returned continuation %q again", page, p.NextToken)
		}
		tokens[p.NextToken] = true
		token = p.NextToken
	}
	if len(seen) == 0 {
		t.Error("no invocation listed")
	}
	if !partial {
		t.Errorf("the stream listing stopped short of %d streams, but no page reported a partial failure (%d of %d invocations shown)", 620, len(seen), len(own))
	}
}

// ─── row 6: SnapStart ──────────────────────────────────────────────────────

// A SnapStart invocation in a new execution environment has no init duration;
// its restore duration marks it a cold start and is shown. A warm invocation
// is not a cold start and shows no restore duration.
func TestLambdaInvocations_SnapStartRestoreIsColdStart(t *testing.T) {
	now := time.Now()
	var textEvents, jsonEvents []cwlogstypes.FilteredLogEvent
	next := 0
	text := lambdaLogWriter{events: &textEvents, next: &next}
	js := lambdaLogWriter{events: &jsonEvents, next: &next}
	restored := lambdaReport{durationMs: "112.84", restoreDurationMs: "474.16", billedRestoreMs: 311, billedMs: 113, memoryMB: 512, usedMB: 141, status: "success"}
	warm := lambdaReport{durationMs: "9.27", billedMs: 10, memoryMB: 512, usedMB: 142, status: "success"}
	const tCold, tWarm = "3d1f5a7b-9c2e-4f6a-8b0d-000000000001", "3d1f5a7b-9c2e-4f6a-8b0d-000000000002"
	const jCold, jWarm = "4e2a6b8c-0d3f-4a7b-9c1e-000000000001", "4e2a6b8c-0d3f-4a7b-9c1e-000000000002"
	at := now.Add(-30 * time.Minute)
	text.textInvocation(defaultStream(at, "5a6b7c8d9e0f41a2b3c4d5e6f7a8b9c0"), tCold, at, restored)
	text.textInvocation(defaultStream(at, "5a6b7c8d9e0f41a2b3c4d5e6f7a8b9c0"), tWarm, at.Add(5*time.Minute), warm)
	js.jsonInvocation(defaultStream(at, "6b7c8d9e0f1a42b3c4d5e6f7a8b9c0d1"), jCold, at, restored)
	js.jsonInvocation(defaultStream(at, "6b7c8d9e0f1a42b3c4d5e6f7a8b9c0d1"), jWarm, at.Add(5*time.Minute), warm)
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-pricing":      {name: "/aws/lambda/acme-pricing", events: textEvents},
		"/aws/lambda/acme-pricing-json": {name: "/aws/lambda/acme-pricing-json", events: jsonEvents},
	}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-pricing", "/aws/lambda/acme-pricing", lambdatypes.LogFormatText),
		lambdaFunction("acme-pricing-json", "/aws/lambda/acme-pricing-json", lambdatypes.LogFormatJson),
	}
	for fn, pair := range map[string][2]string{"acme-pricing": {tCold, tWarm}, "acme-pricing-json": {jCold, jWarm}} {
		rows, _ := openInvocations(t, world, fns, fn)
		cold, hot := rowByRequestID(t, rows, pair[0]), rowByRequestID(t, rows, pair[1])
		if cold.Fields["cold_start"] != "yes" {
			t.Errorf("%s restored invocation: cold_start %q, want yes", fn, cold.Fields["cold_start"])
		}
		if cold.Fields["init_duration_ms"] != "" {
			t.Errorf("%s restored invocation: init_duration_ms %q, want none (SnapStart reports no init duration)", fn, cold.Fields["init_duration_ms"])
		}
		if fieldContaining(cold, "474.16", "request_id") == "" {
			t.Errorf("%s restored invocation shows no restore duration 474.16 (fields %v)", fn, cold.Fields)
		}
		if hot.Fields["cold_start"] != "no" {
			t.Errorf("%s warm invocation: cold_start %q, want no", fn, hot.Fields["cold_start"])
		}
		if k := fieldContaining(hot, "474.16"); k != "" {
			t.Errorf("%s warm invocation carries the other invocation's restore duration in %s", fn, k)
		}
	}
}

// ─── row 7: the window is fixed when the list opens ────────────────────────

// The invocation list covers the 24 hours before it opened. Load More pressed
// later still reaches the oldest of those invocations: the window's lower
// edge travels with the continuation instead of following the clock.
func TestLambdaInvocations_LoadMoreKeepsTheOpeningWindow(t *testing.T) {
	const group = "/aws/lambda/acme-orders-api"
	t0 := time.Now()
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "25.61", billedMs: 26, memoryMB: 128, usedMB: 77, status: "success"}
	const reportOffset = 3011 * time.Millisecond
	edge := t0.Add(-24*time.Hour + 800*time.Millisecond - reportOffset)
	lw.textInvocation(defaultStream(edge, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), "5f3c9a2e-7b1d-4c8e-9a6f-000000000900", edge, ok)
	lw.textInvocation(defaultStream(edge, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), "5f3c9a2e-7b1d-4c8e-9a6f-000000000901", t0.Add(-24*time.Hour-time.Minute-reportOffset), ok)
	for i := range 110 {
		at := t0.Add(-23*time.Hour - 50*time.Minute + time.Duration(i)*13*time.Minute)
		lw.textInvocation(defaultStream(at, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), lambdaRequestID(i), at, ok)
	}
	slices.SortStableFunc(events, func(a, b cwlogstypes.FilteredLogEvent) int { return int(*a.Timestamp - *b.Timestamp) })
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{group: {name: group, events: events}}}
	fns := []lambdatypes.FunctionConfiguration{lambdaFunction("acme-orders-api", group, lambdatypes.LogFormatText)}

	fetch := invocationsFetcher(world, invocationsContext(t, fns, "acme-orders-api"))
	first, err := fetch("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened := time.Since(t0); opened > 700*time.Millisecond {
		t.Skipf("opening took %v; the edge invocation sits only 800ms inside the window", opened)
	}
	if first.Pagination == nil || !first.Pagination.IsTruncated {
		t.Fatalf("first page of %d invocations out of 111 offers no Load More", len(first.Resources))
	}
	time.Sleep(1500 * time.Millisecond)

	shown := requestIDsOf(first.Resources)
	token := first.Pagination.NextToken
	for page := 2; token != ""; page++ {
		if page > 20 {
			t.Fatal("Load More did not end in 20 pages")
		}
		res, err := fetch(token)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		shown = append(shown, requestIDsOf(res.Resources)...)
		token = ""
		if res.Pagination != nil && res.Pagination.IsTruncated {
			token = res.Pagination.NextToken
		}
	}
	want := newestReportIDs(events, t0.Add(-24*time.Hour))
	if !slices.Equal(shown, want) {
		t.Errorf("Load More walk after 1.5s shows %d invocations, want the %d of the 24 hours before opening; missing the oldest: %v",
			len(shown), len(want), slices.DeleteFunc(slices.Clone(want), func(id string) bool { return slices.Contains(shown, id) }))
	}
}

// ─── row 8: JSON platform lines in an invocation's log ─────────────────────

// In an invocation's log, a JSON-format function's platform.start and
// platform.report lines wear the status and findings a text-format
// function's START and REPORT lines wear.
func TestLambdaInvocationLogs_JSONPlatformLinesStyledAsStartAndReport(t *testing.T) {
	now := time.Now()
	var textEvents, jsonEvents []cwlogstypes.FilteredLogEvent
	next := 0
	text := lambdaLogWriter{events: &textEvents, next: &next}
	js := lambdaLogWriter{events: &jsonEvents, next: &next}
	ok := lambdaReport{durationMs: "41.07", billedMs: 42, memoryMB: 128, usedMB: 86, status: "success"}
	at := now.Add(-20 * time.Minute)
	const tid, jid = "0b7c1f3e-2a4d-4e6f-8a9b-000000000201", "7e4d2c1b-9f8a-4b3c-8d2e-000000000201"
	text.textInvocation(defaultStream(at, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), tid, at, ok)
	js.jsonInvocation(defaultStream(at, "f0e9d8c7b6a5948372615049382716a5"), jid, at, ok)
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-orders-api":   {name: "/aws/lambda/acme-orders-api", events: textEvents},
		"/aws/lambda/acme-billing-sync": {name: "/aws/lambda/acme-billing-sync", events: jsonEvents},
	}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-orders-api", "/aws/lambda/acme-orders-api", lambdatypes.LogFormatText),
		lambdaFunction("acme-billing-sync", "/aws/lambda/acme-billing-sync", lambdatypes.LogFormatJson),
	}
	textRows, textCtx := openInvocations(t, world, fns, "acme-orders-api")
	jsonRows, jsonCtx := openInvocations(t, world, fns, "acme-billing-sync")
	textLog := openInvocationLog(t, world, rowByRequestID(t, textRows, tid), textCtx)
	jsonLog := openInvocationLog(t, world, rowByRequestID(t, jsonRows, jid), jsonCtx)

	line := func(rows []resource.Resource, marker string) resource.Resource {
		t.Helper()
		for _, r := range rows {
			if strings.Contains(r.Fields["message"], marker) {
				return r
			}
		}
		t.Fatalf("no log line holding %q", marker)
		return resource.Resource{}
	}
	for _, pair := range [][2]string{{"START RequestId", `"type":"platform.start"`}, {"REPORT RequestId", `"type":"platform.report"`}} {
		tr, jr := line(textLog, pair[0]), line(jsonLog, pair[1])
		if tr.Fields["status"] == "" {
			t.Fatalf("text %s line has no status; the comparison needs a styled text line", pair[0])
		}
		if jr.Fields["status"] != tr.Fields["status"] || !slices.Equal(findingCodes(jr.Findings), findingCodes(tr.Findings)) {
			t.Errorf("JSON %s line: status %q findings %v, want %q %v as the text %s line",
				pair[1], jr.Fields["status"], findingCodes(jr.Findings), tr.Fields["status"], findingCodes(tr.Findings), pair[0])
		}
	}
}
