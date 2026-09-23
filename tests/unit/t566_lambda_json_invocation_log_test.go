// t566_lambda_json_invocation_log_test.go — an invocation's log in the JSON
// log format is bounded by its platform.start and platform.report records.
package unit

import (
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestLambdaInvocationLogs_JSONFormatShowsTheWholeInvocation(t *testing.T) {
	now := time.Now()
	stream := defaultStream(now.Add(-time.Hour), "c0ffee00c0ffee00c0ffee00c0ffee00")
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "12.40", billedMs: 13, memoryMB: 128, usedMB: 80, status: "success"}
	const target = "5c5c5c5c-6d6d-4e7e-8f8f-000000000002"

	at := now.Add(-30 * time.Minute)
	lw.jsonInvocation(stream, "5c5c5c5c-6d6d-4e7e-8f8f-000000000001", at.Add(-time.Minute), ok)
	first := len(events)
	lw.jsonInvocation(stream, target, at, ok)
	lw.write(stream, at.Add(10*time.Millisecond), "Traceback (most recent call last):\n  File \"/var/task/app.py\", line 7, in handler\nValueError: bad batch\n")
	want := slices.Clone(events[first:])
	lw.jsonInvocation(stream, "5c5c5c5c-6d6d-4e7e-8f8f-000000000003", at.Add(time.Minute), ok)
	sort.SliceStable(events, func(i, j int) bool { return aws.ToInt64(events[i].Timestamp) < aws.ToInt64(events[j].Timestamp) })

	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-billing-sync": {name: "/aws/lambda/acme-billing-sync", events: events},
	}}
	fns := []lambdatypes.FunctionConfiguration{lambdaFunction("acme-billing-sync", "/aws/lambda/acme-billing-sync", lambdatypes.LogFormatJson)}
	rows, pctx := openInvocations(t, world, fns, "acme-billing-sync")
	i := slices.IndexFunc(rows, func(r resource.Resource) bool { return r.Fields["request_id"] == target })
	if i < 0 {
		t.Fatalf("invocation %s not listed (%v)", target, requestIDsOf(rows))
	}
	var got, wantKeys []string
	for _, r := range openInvocationLog(t, world, rows[i], pctx) {
		got = append(got, logRowKey(t, r))
	}
	for _, e := range want {
		wantKeys = append(wantKeys, strconv.FormatInt(aws.ToInt64(e.Timestamp), 10)+"|"+aws.ToString(e.Message))
	}
	slices.Sort(got)
	slices.Sort(wantKeys)
	if !slices.Equal(got, wantKeys) {
		t.Errorf("invocation log shows %d lines, want its %d from platform.start to platform.report", len(got), len(wantKeys))
	}
}

// A stream opened days before the lookback window (a long-lived execution
// environment) still holds invocations inside it; the function lists them,
// and not those of another function sharing the group.
func TestLambdaInvocations_SharedGroupStreamOpenedBeforeTheWindow(t *testing.T) {
	const shared = "/acme/lambda/orders"
	now := time.Now()
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "25.61", billedMs: 26, memoryMB: 128, usedMB: 77, status: "success"}
	old := now.Add(-72 * time.Hour)
	var mine []string
	for i := range 6 {
		rid := "6e6e6e6e-7f7f-4a8a-9b9b-00000000000" + strconv.Itoa(i)
		fn := []string{"acme-orders", "acme-orders-api"}[i%2]
		lw.textInvocation(customStream(old, fn, "d1d2d3d4e5e6f7f8a9a0b1b2c3c4d5d6"), rid, now.Add(-time.Duration(6-i)*10*time.Minute), ok)
		if fn == "acme-orders" {
			mine = append([]string{rid}, mine...)
		}
	}
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{shared: {name: shared, events: events}}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-orders", shared, lambdatypes.LogFormatText),
		lambdaFunction("acme-orders-api", shared, lambdatypes.LogFormatText),
	}
	rows, _ := openInvocations(t, world, fns, "acme-orders")
	if got := requestIDsOf(rows); !slices.Equal(got, mine) {
		t.Errorf("acme-orders lists %v, want %v", got, mine)
	}
}

// A function that switched from the text to the JSON log format keeps its
// older invocations in text: the list holds both, newest first.
func TestLambdaInvocations_FormatSwitchListsBothFormats(t *testing.T) {
	now := time.Now()
	stream := defaultStream(now.Add(-3*time.Hour), "e1e2e3e4f5f6a7a8b9b0c1c2d3d4e5e6")
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "31.20", billedMs: 32, memoryMB: 128, usedMB: 70, status: "success"}
	var want []string
	for i := range 4 {
		rid := "8f8f8f8f-9a9a-4b0b-8c1c-00000000000" + strconv.Itoa(i)
		at := now.Add(-time.Duration(4-i) * 30 * time.Minute)
		if i < 2 {
			lw.textInvocation(stream, rid, at, ok)
		} else {
			lw.jsonInvocation(stream, rid, at, ok)
		}
		want = append([]string{rid}, want...)
	}
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-billing-sync": {name: "/aws/lambda/acme-billing-sync", events: events},
	}}
	fns := []lambdatypes.FunctionConfiguration{lambdaFunction("acme-billing-sync", "/aws/lambda/acme-billing-sync", lambdatypes.LogFormatJson)}
	rows, _ := openInvocations(t, world, fns, "acme-billing-sync")
	if got := requestIDsOf(rows); !slices.Equal(got, want) {
		t.Errorf("lists %v, want both formats newest first %v", got, want)
	}
}
