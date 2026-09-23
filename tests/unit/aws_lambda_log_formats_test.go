// aws_lambda_log_formats_test.go — the Lambda invocation list and an
// invocation's log over the log formats and log groups Lambda writes.
//
// Shapes, as AWS documents them:
//
//   - JSON log format (docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-logformat.html):
//     each system log event is one JSON object {"time", "type", "record"};
//     application log events are {"timestamp", "level", "message", "requestId"}.
//   - platform.report (docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-report):
//     record = {requestId, status, metrics{durationMs, billedDurationMs,
//     memorySizeMB, maxMemoryUsedMB, initDurationMs?}}; status is one of
//     success|failure|error|timeout.
//   - Custom log groups (docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-loggroups.html):
//     several functions may share one group; their streams are named
//     YYYY/MM/DD/<function_name>[<function_version>][<execution_environment_GUID>].
//     A function in its default group /aws/lambda/<name> writes
//     YYYY/MM/DD/[<function_version>]<execution_environment_GUID>.
//
// A function's lines that its runtime writes without the request ID (a
// Python print, an uncaught stack trace) sit in the invocation's stream
// between its START and REPORT.
package unit

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─── JSON filter patterns ──────────────────────────────────────────────────

type jsonPatternClause struct {
	path  []string
	neq   bool
	value string
}

// parseJSONLogPattern reads a CloudWatch Logs JSON filter pattern of the form
// { $.a.b = "v" && $.c != 1 }. isJSON is false for any other pattern; a
// braced pattern this fake does not model is an error.
func parseJSONLogPattern(pattern string) (clauses []jsonPatternClause, isJSON bool, err error) {
	p := strings.TrimSpace(pattern)
	if !strings.HasPrefix(p, "{") {
		return nil, false, nil
	}
	if !strings.HasSuffix(p, "}") || strings.Contains(p, "||") {
		return nil, true, fmt.Errorf("fake FilterLogEvents: JSON filter pattern %q is not modelled", pattern)
	}
	for _, part := range strings.Split(strings.TrimSpace(p[1:len(p)-1]), "&&") {
		part = strings.TrimSpace(part)
		op := "="
		if strings.Contains(part, "!=") {
			op = "!="
		}
		lhs, rhs, ok := strings.Cut(part, op)
		lhs, rhs = strings.TrimSpace(lhs), strings.TrimSpace(rhs)
		if !ok || !strings.HasPrefix(lhs, "$.") || rhs == "" || strings.ContainsAny(rhs, "*%") {
			return nil, true, fmt.Errorf("fake FilterLogEvents: JSON filter clause %q is not modelled", part)
		}
		clauses = append(clauses, jsonPatternClause{path: strings.Split(lhs[2:], "."), neq: op == "!=", value: strings.Trim(rhs, `"`)})
	}
	return clauses, true, nil
}

func jsonLogPatternMatches(clauses []jsonPatternClause, msg string) bool {
	var doc any
	if json.Unmarshal([]byte(msg), &doc) != nil {
		return false
	}
	for _, c := range clauses {
		v := doc
		for _, k := range c.path {
			m, ok := v.(map[string]any)
			if !ok {
				v = nil
				break
			}
			v = m[k]
		}
		got, present := "", v != nil
		switch x := v.(type) {
		case string:
			got = x
		case float64:
			got = strconv.FormatFloat(x, 'f', -1, 64)
		case bool:
			got = strconv.FormatBool(x)
		}
		if c.neq {
			if present && got == c.value {
				return false
			}
		} else if !present || got != c.value {
			return false
		}
	}
	return true
}

// ─── a CloudWatch Logs account holding several log groups ──────────────────

// lambdaLogWorld is the CloudWatch Logs API over several log groups.
// FilterLogEvents is fakeLogGroup's (window, stream names or prefix,
// pattern, Limit, oldest first). DescribeLogStreams honours
// logStreamNamePrefix, orderBy, descending and limit, and refuses a prefix
// with orderBy LastEventTime as AWS does. GetLogEvents reads one stream
// inside [startTime, endTime).
type lambdaLogWorld struct {
	groups map[string]*fakeLogGroup
}

var _ awsclient.CWLogsAPI = (*lambdaLogWorld)(nil)

func (w *lambdaLogWorld) group(name, identifier *string) (*fakeLogGroup, error) {
	n := aws.ToString(name)
	if n == "" {
		n = aws.ToString(identifier)
	}
	g, ok := w.groups[n]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "The specified log group does not exist."}
	}
	return g, nil
}

func (w *lambdaLogWorld) FilterLogEvents(ctx context.Context, in *cloudwatchlogs.FilterLogEventsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	g, err := w.group(in.LogGroupName, in.LogGroupIdentifier)
	if err != nil {
		return nil, err
	}
	return g.FilterLogEvents(ctx, in, opts...)
}

func (w *lambdaLogWorld) DescribeLogStreams(_ context.Context, in *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	g, err := w.group(in.LogGroupName, in.LogGroupIdentifier)
	if err != nil {
		return nil, err
	}
	if in.LogStreamNamePrefix != nil && in.OrderBy == cwlogstypes.OrderByLastEventTime {
		return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "Cannot order by LastEventTime with a logStreamNamePrefix."}
	}
	byName := map[string]*cwlogstypes.LogStream{}
	for _, e := range g.events {
		name := aws.ToString(e.LogStreamName)
		if !strings.HasPrefix(name, aws.ToString(in.LogStreamNamePrefix)) {
			continue
		}
		s, ok := byName[name]
		if !ok {
			s = &cwlogstypes.LogStream{
				LogStreamName:       aws.String(name),
				Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + g.name + ":log-stream:" + name),
				CreationTime:        e.Timestamp,
				FirstEventTimestamp: e.Timestamp,
				LastEventTimestamp:  e.Timestamp,
			}
			byName[name] = s
		}
		ts := aws.ToInt64(e.Timestamp)
		if ts < aws.ToInt64(s.FirstEventTimestamp) {
			s.FirstEventTimestamp, s.CreationTime = e.Timestamp, e.Timestamp
		}
		if ts > aws.ToInt64(s.LastEventTimestamp) {
			s.LastEventTimestamp = e.Timestamp
			s.LastIngestionTime = e.IngestionTime
		}
	}
	streams := make([]cwlogstypes.LogStream, 0, len(byName))
	for _, s := range byName {
		streams = append(streams, *s)
	}
	sort.Slice(streams, func(i, j int) bool {
		a, b := streams[i], streams[j]
		if aws.ToBool(in.Descending) {
			a, b = b, a
		}
		if in.OrderBy == cwlogstypes.OrderByLastEventTime {
			return aws.ToInt64(a.LastEventTimestamp) < aws.ToInt64(b.LastEventTimestamp)
		}
		return aws.ToString(a.LogStreamName) < aws.ToString(b.LogStreamName)
	})
	limit := 50
	if in.Limit != nil {
		limit = int(*in.Limit)
	}
	offset := 0
	if in.NextToken != nil {
		offset, err = strconv.Atoi(strings.TrimPrefix(*in.NextToken, "streams/"))
		if err != nil {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "The specified nextToken is invalid."}
		}
	}
	end := min(offset+limit, len(streams))
	out := &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: streams[min(offset, end):end]}
	if end < len(streams) {
		out.NextToken = aws.String("streams/" + strconv.Itoa(end))
	}
	return out, nil
}

func (w *lambdaLogWorld) GetLogEvents(_ context.Context, in *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	g, err := w.group(in.LogGroupName, in.LogGroupIdentifier)
	if err != nil {
		return nil, err
	}
	var events []cwlogstypes.OutputLogEvent
	for _, e := range g.events {
		ts := aws.ToInt64(e.Timestamp)
		if aws.ToString(e.LogStreamName) != aws.ToString(in.LogStreamName) ||
			(in.StartTime != nil && ts < *in.StartTime) || (in.EndTime != nil && ts >= *in.EndTime) {
			continue
		}
		events = append(events, cwlogstypes.OutputLogEvent{Timestamp: e.Timestamp, IngestionTime: e.IngestionTime, Message: e.Message})
	}
	if len(events) == 0 && !slices.ContainsFunc(g.events, func(e cwlogstypes.FilteredLogEvent) bool {
		return aws.ToString(e.LogStreamName) == aws.ToString(in.LogStreamName)
	}) {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "The specified log stream does not exist."}
	}
	if len(events) > 10000 {
		return nil, fmt.Errorf("fake GetLogEvents: a window of %d events is not modelled", len(events))
	}
	end := "f/" + strconv.Itoa(len(events))
	if tok := aws.ToString(in.NextToken); tok != "" {
		if tok != end && tok != "b/0" {
			return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "The specified nextToken is invalid."}
		}
		return &cloudwatchlogs.GetLogEventsOutput{NextForwardToken: aws.String(end), NextBackwardToken: aws.String("b/0")}, nil
	}
	return &cloudwatchlogs.GetLogEventsOutput{Events: events, NextForwardToken: aws.String(end), NextBackwardToken: aws.String("b/0")}, nil
}

func (w *lambdaLogWorld) DescribeLogGroups(_ context.Context, in *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	out := &cloudwatchlogs.DescribeLogGroupsOutput{}
	for name := range w.groups {
		if strings.HasPrefix(name, aws.ToString(in.LogGroupNamePrefix)) {
			out.LogGroups = append(out.LogGroups, cwlogstypes.LogGroup{
				LogGroupName: aws.String(name),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name + ":*"),
			})
		}
	}
	return out, nil
}

func (w *lambdaLogWorld) DescribeMetricFilters(_ context.Context, _ *cloudwatchlogs.DescribeMetricFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	return &cloudwatchlogs.DescribeMetricFiltersOutput{}, nil
}

// ─── Lambda log lines ──────────────────────────────────────────────────────

type lambdaReport struct {
	durationMs, initDurationMs string // as written: "693.92", "" for no init
	restoreDurationMs          string // SnapStart restore, "" when none
	billedRestoreMs            int
	billedMs, memoryMB, usedMB int
	status                     string // success | failure | error | timeout
	errorType                  string // with failure or error
}

// lambdaLogWriter appends the lines a runtime writes to one stream.
type lambdaLogWriter struct {
	events *[]cwlogstypes.FilteredLogEvent
	next   *int
}

func (lw lambdaLogWriter) write(stream string, at time.Time, msg string) {
	*lw.events = append(*lw.events, logEventAt(*lw.next, at, stream, msg))
	*lw.next++
}

// textInvocation writes the plain-text lines of one invocation: START, one
// application line carrying the request ID, END and REPORT.
func (lw lambdaLogWriter) textInvocation(stream, rid string, at time.Time, r lambdaReport) {
	lw.write(stream, at, "START RequestId: "+rid+" Version: $LATEST\n")
	lw.write(stream, at.Add(4*time.Millisecond), "[INFO]\t"+at.UTC().Format("2006-01-02T15:04:05.000Z")+"\t"+rid+"\torder accepted\n")
	if r.status == "timeout" {
		lw.write(stream, at.Add(3000*time.Millisecond), at.UTC().Format("2006-01-02T15:04:05.000Z")+" "+rid+" Task timed out after 3.00 seconds\n")
	}
	end := at.Add(3010 * time.Millisecond)
	lw.write(stream, end, "END RequestId: "+rid+"\n")
	line := fmt.Sprintf("REPORT RequestId: %s\tDuration: %s ms\tBilled Duration: %d ms\tMemory Size: %d MB\tMax Memory Used: %d MB\t", rid, r.durationMs, r.billedMs, r.memoryMB, r.usedMB)
	if r.initDurationMs != "" {
		line += "Init Duration: " + r.initDurationMs + " ms\t"
	}
	if r.restoreDurationMs != "" {
		line += fmt.Sprintf("Restore Duration: %s ms\tBilled Restore Duration: %d ms\t", r.restoreDurationMs, r.billedRestoreMs)
	}
	if r.status != "" && r.status != "success" {
		line += "Status: " + r.status + "\t"
	}
	if r.errorType != "" {
		line += "Error Type: " + r.errorType + "\t"
	}
	lw.write(stream, end.Add(time.Millisecond), strings.TrimSuffix(line, "\t")+"\n")
}

func lambdaJSONTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// jsonInvocation writes the JSON-format lines of one invocation:
// platform.start, one application record, platform.runtimeDone and
// platform.report, shaped as the telemetry schema reference documents.
func (lw lambdaLogWriter) jsonInvocation(stream, rid string, at time.Time, r lambdaReport) {
	num := func(s string) json.Number { return json.Number(s) }
	lw.write(stream, at, mustJSON(map[string]any{
		"time": lambdaJSONTime(at), "type": "platform.start",
		"record": map[string]any{"requestId": rid, "version": "$LATEST"},
	}))
	lw.write(stream, at.Add(4*time.Millisecond), mustJSON(map[string]any{
		"timestamp": lambdaJSONTime(at.Add(4 * time.Millisecond)), "level": "INFO", "message": "billing batch synced", "requestId": rid,
	}))
	end := at.Add(3010 * time.Millisecond)
	lw.write(stream, end, mustJSON(map[string]any{
		"time": lambdaJSONTime(end), "type": "platform.runtimeDone",
		"record": map[string]any{"requestId": rid, "status": r.status, "metrics": map[string]any{"durationMs": num(r.durationMs), "producedBytes": 52}},
	}))
	metrics := map[string]any{
		"durationMs": num(r.durationMs), "billedDurationMs": r.billedMs,
		"memorySizeMB": r.memoryMB, "maxMemoryUsedMB": r.usedMB,
	}
	if r.initDurationMs != "" {
		metrics["initDurationMs"] = num(r.initDurationMs)
	}
	if r.restoreDurationMs != "" {
		metrics["restoreDurationMs"] = num(r.restoreDurationMs)
	}
	record := map[string]any{"requestId": rid, "status": r.status, "metrics": metrics}
	if r.errorType != "" {
		record["errorType"] = r.errorType
	}
	lw.write(stream, end.Add(time.Millisecond), mustJSON(map[string]any{
		"time": lambdaJSONTime(end.Add(time.Millisecond)), "type": "platform.report",
		"record": record,
	}))
}

func defaultStream(at time.Time, guid string) string {
	return at.UTC().Format("2006/01/02") + "/[$LATEST]" + guid
}

func customStream(at time.Time, function, guid string) string {
	return at.UTC().Format("2006/01/02") + "/" + function + "[$LATEST][" + guid + "]"
}

// ─── driving the app: function list → invocation list → invocation log ────

func lambdaFunction(name, group string, format lambdatypes.LogFormat) lambdatypes.FunctionConfiguration {
	return lambdatypes.FunctionConfiguration{
		FunctionName: aws.String(name),
		FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
		Runtime:      lambdatypes.RuntimePython312,
		Handler:      aws.String("app.handler"),
		MemorySize:   aws.Int32(128),
		Timeout:      aws.Int32(3),
		LastModified: aws.String("2026-09-01T10:00:00.000+0000"),
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogFormat: format,
			LogGroup:  aws.String(group),
		},
	}
}

// invocationsContext lists the functions, selects name, and resolves the
// invocation list's child context from its row the way the catalog does.
func invocationsContext(t *testing.T, fns []lambdatypes.FunctionConfiguration, name string) map[string]string {
	t.Helper()
	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), &mockLambdaListFunctionsClient{output: &lambda.ListFunctionsOutput{Functions: fns}}, "")
	if err != nil {
		t.Fatalf("ListFunctions: %v", err)
	}
	var row *resource.Resource
	for i := range page.Resources {
		if page.Resources[i].Fields["function_name"] == name {
			row = &page.Resources[i]
		}
	}
	if row == nil {
		t.Fatalf("function %q not in the list", name)
	}
	def := resource.FindResourceType("lambda")
	for i := range def.Children {
		if def.Children[i].ChildType == "lambda_invocations" {
			return resource.ResolveChildContext(def.Children[i], row, nil)
		}
	}
	t.Fatal("lambda has no lambda_invocations child")
	return nil
}

// openInvocations opens name's invocation list and returns its first page.
func openInvocations(t *testing.T, world *lambdaLogWorld, fns []lambdatypes.FunctionConfiguration, name string) ([]resource.Resource, map[string]string) {
	t.Helper()
	pctx := invocationsContext(t, fns, name)
	res, err := resource.GetChildType("lambda_invocations").ChildFetcher(context.Background(), &awsclient.ServiceClients{CloudWatchLogs: world}, pctx, "")
	if err != nil {
		t.Fatalf("%s invocations: %v", name, err)
	}
	return res.Resources, pctx
}

func openInvocationLog(t *testing.T, world *lambdaLogWorld, invocation resource.Resource, pctx map[string]string) []resource.Resource {
	t.Helper()
	ctd := resource.GetChildType("lambda_invocations")
	if len(ctd.Children) == 0 {
		t.Fatal("lambda_invocations has no child view")
	}
	lctx := resource.ResolveChildContext(ctd.Children[0], &invocation, pctx)
	res, err := resource.GetChildType(ctd.Children[0].ChildType).ChildFetcher(context.Background(), &awsclient.ServiceClients{CloudWatchLogs: world}, lctx, "")
	if err != nil {
		t.Fatalf("invocation log of %s: %v", invocation.ID, err)
	}
	return res.Resources
}

func requestIDsOf(rows []resource.Resource) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Fields["request_id"])
	}
	return ids
}

// ─── row 1: JSON log format ────────────────────────────────────────────────

var lambdaReports = []lambdaReport{ //nolint:gochecknoglobals // read-only fixture
	{durationMs: "693.92", initDurationMs: "397.68", billedMs: 694, memoryMB: 128, usedMB: 84, status: "success"},
	{durationMs: "41.07", billedMs: 42, memoryMB: 128, usedMB: 86, status: "success"},
	{durationMs: "3003.52", billedMs: 3000, memoryMB: 128, usedMB: 90, status: "timeout"},
}

// A JSON-format function lists its invocations from platform.report records
// with the same values a text-format function's REPORT lines give, and a
// record whose status is timeout is flagged as a text-format timeout is.
func TestLambdaInvocations_JSONFormatListsReportRecords(t *testing.T) {
	now := time.Now()
	var textEvents, jsonEvents []cwlogstypes.FilteredLogEvent
	next := 0
	text := lambdaLogWriter{events: &textEvents, next: &next}
	js := lambdaLogWriter{events: &jsonEvents, next: &next}
	var textIDs, jsonIDs []string
	for i, r := range lambdaReports {
		at := now.Add(-time.Duration(3-i) * 40 * time.Minute)
		tid := fmt.Sprintf("0b7c1f3e-2a4d-4e6f-8a9b-%012d", i+1)
		jid := fmt.Sprintf("7e4d2c1b-9f8a-4b3c-8d2e-%012d", i+1)
		text.textInvocation(defaultStream(at, "a1b2c3d4e5f60718293a4b5c6d7e8f90"), tid, at, r)
		js.jsonInvocation(defaultStream(at, "f0e9d8c7b6a5948372615049382716a5"), jid, at.Add(time.Second), r)
		textIDs = append([]string{tid}, textIDs...)
		jsonIDs = append([]string{jid}, jsonIDs...)
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

	if got := requestIDsOf(textRows); !slices.Equal(got, textIDs) {
		t.Fatalf("text-format function lists %v, want %v", got, textIDs)
	}
	if got := requestIDsOf(jsonRows); !slices.Equal(got, jsonIDs) {
		t.Fatalf("JSON-format function lists %v, want its three invocations newest first %v", got, jsonIDs)
	}

	wantText := []map[string]string{
		{"duration_ms": "3003.52 ms", "billed_duration_ms": "3000 ms", "memory_size_mb": "128", "memory_used_mb": "90", "init_duration_ms": ""},
		{"duration_ms": "41.07 ms", "billed_duration_ms": "42 ms", "memory_size_mb": "128", "memory_used_mb": "86", "init_duration_ms": ""},
		{"duration_ms": "693.92 ms", "billed_duration_ms": "694 ms", "memory_size_mb": "128", "memory_used_mb": "84", "init_duration_ms": "397.68 ms"},
	}
	keys := []string{"duration_ms", "duration_ms_raw", "billed_duration_ms", "billed_duration_ms_raw", "memory_size_mb", "memory_used_mb", "memory_used", "init_duration_ms", "cold_start", "status"}
	for i := range textRows {
		for k, v := range wantText[i] {
			if textRows[i].Fields[k] != v {
				t.Errorf("text invocation %s: %s = %q, want %q", textRows[i].ID, k, textRows[i].Fields[k], v)
			}
		}
		for _, k := range keys {
			if jsonRows[i].Fields[k] != textRows[i].Fields[k] {
				t.Errorf("JSON invocation %s: %s = %q, want %q as the same report reads in text format",
					jsonRows[i].ID, k, jsonRows[i].Fields[k], textRows[i].Fields[k])
			}
		}
		if got, want := findingCodes(jsonRows[i].Findings), findingCodes(textRows[i].Findings); !slices.Equal(got, want) {
			t.Errorf("JSON invocation %s findings %v, want %v as the same report gives in text format", jsonRows[i].ID, got, want)
		}
	}
	if got := findingCodes(textRows[0].Findings); !slices.Equal(got, []string{string(awsclient.CodeLambdaInvocationTimeout)}) {
		t.Errorf("text timeout findings %v, want [%s]", got, awsclient.CodeLambdaInvocationTimeout)
	}
}

// ─── row 2: an invocation's own stream ─────────────────────────────────────

func logRowKey(t *testing.T, r resource.Resource) string {
	t.Helper()
	switch raw := r.RawStruct.(type) {
	case cwlogstypes.FilteredLogEvent:
		return strconv.FormatInt(aws.ToInt64(raw.Timestamp), 10) + "|" + aws.ToString(raw.Message)
	case cwlogstypes.OutputLogEvent:
		return strconv.FormatInt(aws.ToInt64(raw.Timestamp), 10) + "|" + aws.ToString(raw.Message)
	}
	t.Fatalf("log row %q RawStruct is %T, want a CloudWatch Logs event", r.ID, r.RawStruct)
	return ""
}

// An invocation's log is its stream from START to REPORT: every line the
// runtime wrote in between, including those without the request ID, and
// nothing of the invocations before and after it on the same stream or of a
// concurrent invocation on another stream.
func TestLambdaInvocationLogs_ShowsTheWholeInvocationFromItsStream(t *testing.T) {
	now := time.Now()
	s1 := defaultStream(now.Add(-time.Hour), "a1b2c3d4e5f60718293a4b5c6d7e8f90")
	s2 := defaultStream(now.Add(-time.Hour), "0f1e2d3c4b5a69788796a5b4c3d2e1f0")
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "12.40", billedMs: 13, memoryMB: 128, usedMB: 80, status: "success"}

	const before, target, after, concurrent = "11111111-2222-4333-8444-000000000001", "11111111-2222-4333-8444-000000000002",
		"11111111-2222-4333-8444-000000000003", "11111111-2222-4333-8444-000000000004"
	b := now.Add(-50 * time.Minute)
	lw.textInvocation(s1, before, b.Add(-2*time.Minute), ok)

	first := len(events)
	lw.write(s1, b, "START RequestId: "+target+" Version: $LATEST\n")
	lw.write(s1, b.Add(5*time.Millisecond), "processing order 5001\n")
	lw.write(s1, b.Add(40*time.Millisecond), "Traceback (most recent call last):\n  File \"/var/task/app.py\", line 42, in handler\n    sku = order[\"sku\"]\nKeyError: 'sku'\n")
	lw.write(s1, b.Add(41*time.Millisecond), "[ERROR] KeyError: 'sku'\n")
	lw.write(s1, b.Add(900*time.Millisecond), "END RequestId: "+target+"\n")
	lw.write(s1, b.Add(901*time.Millisecond), "REPORT RequestId: "+target+"\tDuration: 899.18 ms\tBilled Duration: 900 ms\tMemory Size: 128 MB\tMax Memory Used: 81 MB\t\n")
	want := slices.Clone(events[first:])

	lw.write(s2, b.Add(100*time.Millisecond), "START RequestId: "+concurrent+" Version: $LATEST\n")
	lw.write(s2, b.Add(300*time.Millisecond), "processing order 7002\n")
	lw.write(s2, b.Add(500*time.Millisecond), "END RequestId: "+concurrent+"\n")
	lw.write(s2, b.Add(501*time.Millisecond), "REPORT RequestId: "+concurrent+"\tDuration: 400.02 ms\tBilled Duration: 401 ms\tMemory Size: 128 MB\tMax Memory Used: 79 MB\t\n")
	lw.textInvocation(s1, after, b.Add(2*time.Minute), ok)
	sort.SliceStable(events, func(i, j int) bool { return aws.ToInt64(events[i].Timestamp) < aws.ToInt64(events[j].Timestamp) })

	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{
		"/aws/lambda/acme-orders-api": {name: "/aws/lambda/acme-orders-api", events: events},
	}}
	fns := []lambdatypes.FunctionConfiguration{lambdaFunction("acme-orders-api", "/aws/lambda/acme-orders-api", lambdatypes.LogFormatText)}
	rows, pctx := openInvocations(t, world, fns, "acme-orders-api")
	var inv *resource.Resource
	for i := range rows {
		if rows[i].Fields["request_id"] == target {
			inv = &rows[i]
		}
	}
	if inv == nil {
		t.Fatalf("invocation %s not listed (%v)", target, requestIDsOf(rows))
	}

	logRows := openInvocationLog(t, world, *inv, pctx)
	var got, wantKeys []string
	for _, r := range logRows {
		got = append(got, logRowKey(t, r))
	}
	for _, e := range want {
		wantKeys = append(wantKeys, strconv.FormatInt(aws.ToInt64(e.Timestamp), 10)+"|"+aws.ToString(e.Message))
	}
	slices.Sort(got)
	slices.Sort(wantKeys)
	if !slices.Equal(got, wantKeys) {
		t.Errorf("invocation log shows %d lines:\n  %s\nwant its %d lines from START to REPORT:\n  %s",
			len(got), strings.Join(got, "\n  "), len(wantKeys), strings.Join(wantKeys, "\n  "))
	}
}

// ─── row 3: a log group shared by two functions ────────────────────────────

// Two functions writing to one custom log group — one's name a prefix of the
// other's — each list only their own invocations.
func TestLambdaInvocations_SharedLogGroupListsOnlyTheFunctionsOwn(t *testing.T) {
	const shared = "/acme/lambda/orders"
	now := time.Now()
	var events []cwlogstypes.FilteredLogEvent
	next := 0
	lw := lambdaLogWriter{events: &events, next: &next}
	ok := lambdaReport{durationMs: "25.61", billedMs: 26, memoryMB: 128, usedMB: 77, status: "success"}
	own := map[string][]string{}
	for i := range 40 {
		fn := "acme-orders"
		guid := []string{"1a2b3c4d5e6f40718293a4b5c6d7e8f9", "2b3c4d5e6f7a48192a3b4c5d6e7f8091"}[i%4/2]
		if i%2 == 1 {
			fn = "acme-orders-api"
			guid = []string{"3c4d5e6f7a8b492a3b4c5d6e7f8091a2", "4d5e6f7a8b9c4a3b4c5d6e7f8091a2b3"}[i%4/2]
		}
		at := now.Add(-time.Duration(40-i) * 25 * time.Minute)
		streamStart := now.Add(-17 * time.Hour)
		if i >= 20 {
			streamStart = now.Add(-8 * time.Hour)
		}
		rid := fmt.Sprintf("9a8b7c6d-5e4f-4a3b-9c2d-%012d", i)
		lw.textInvocation(customStream(streamStart, fn, guid), rid, at, ok)
		own[fn] = append([]string{rid}, own[fn]...)
	}
	world := &lambdaLogWorld{groups: map[string]*fakeLogGroup{shared: {name: shared, events: events}}}
	fns := []lambdatypes.FunctionConfiguration{
		lambdaFunction("acme-orders", shared, lambdatypes.LogFormatText),
		lambdaFunction("acme-orders-api", shared, lambdatypes.LogFormatText),
	}

	for _, fn := range []string{"acme-orders", "acme-orders-api"} {
		rows, _ := openInvocations(t, world, fns, fn)
		if got := requestIDsOf(rows); !slices.Equal(got, own[fn]) {
			t.Errorf("%s lists %d invocations %v…, want its own %d newest first %v…",
				fn, len(got), got[:min(3, len(got))], len(own[fn]), own[fn][:3])
		}
	}
}
