// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// maxLogScanCalls caps the FilterLogEvents calls one newestLogEvents read
// makes per source. CloudWatch Logs answers a page with no events and a
// NextToken when a call runs out of search time before the window is done,
// so an event count alone never stops a read over a large, quiet group.
const maxLogScanCalls = 100

// newestLogFirstSpan is the width of the first window newestLogEvents reads
// back from its end; each further window doubles it.
const newestLogFirstSpan = 5 * time.Minute

// logSource is one place a view's lines are written: a log group, narrowed to
// the streams under streamPrefix, or to the named streams, and to the events
// matching pattern, when set. region is the Region the group is read in, ""
// for the session's.
type logSource struct {
	api          CWLogsFilterLogEventsAPI
	region       string
	group        string
	streamPrefix string
	streams      []string
	pattern      string
}

// sourcedLogEvent is an event with the log group, and the Region when not
// the session's, it was read from.
type sourcedLogEvent struct {
	cwlogstypes.FilteredLogEvent
	region string
	group  string
}

// logCursor is where a newestLogEvents read starts: the newest millisecond it
// reads and the width of its first window (0 for newestLogFirstSpan), and the
// oldest millisecond of the list's window (start, 0 for none), fixed when the
// list opens so Load More pressed later reaches the same oldest events. When
// one source's single millisecond did not fit a read's call budget, resume is
// that source's FilterLogEvents NextToken inside it and source its index, so
// the next read continues the millisecond instead of starting it over. Load
// More carries it as "<end>", "<end>/<span>" or
// "<end>/<span>/<source>/<resume>", after "<start>," when start is set.
type logCursor struct {
	start, end, span int64
	source           int
	resume           string
}

func (c logCursor) String() string {
	lower := ""
	if c.start != 0 {
		lower = strconv.FormatInt(c.start, 10) + ","
	}
	switch {
	case c.resume != "":
		return fmt.Sprintf("%s%d/%d/%d/%s", lower, c.end, c.span, c.source, c.resume)
	case c.span != 0:
		return fmt.Sprintf("%s%d/%d", lower, c.end, c.span)
	}
	return lower + strconv.FormatInt(c.end, 10)
}

// parseLogCursor reads a Load More token back, or for "" opens a list at now
// whose window reaches lookback into the past (0 for no lower edge).
func parseLogCursor(continuationToken string, lookback time.Duration) (logCursor, error) {
	if continuationToken == "" {
		now := time.Now()
		c := logCursor{end: now.UnixMilli()}
		if lookback > 0 {
			c.start = now.Add(-lookback).UnixMilli()
		}
		return c, nil
	}
	var c logCursor
	var err error
	position := continuationToken
	if lower, rest, ok := strings.Cut(continuationToken, ","); ok && !strings.Contains(lower, "/") {
		c.start, err = strconv.ParseInt(lower, 10, 64)
		position = rest
	}
	parts := strings.SplitN(position, "/", 4)
	if err == nil {
		c.end, err = strconv.ParseInt(parts[0], 10, 64)
	}
	if err == nil && len(parts) > 1 {
		c.span, err = strconv.ParseInt(parts[1], 10, 64)
	}
	if err == nil && len(parts) == 4 {
		c.source, err = strconv.Atoi(parts[2])
		c.resume = parts[3]
	}
	if err != nil || c.start < 0 || c.span < 0 || c.source < 0 || len(parts) == 3 || len(parts) == 4 && c.resume == "" {
		return logCursor{}, fmt.Errorf("continuation token %q is not a log read position", continuationToken)
	}
	return c, nil
}

// newestLogEvents returns the newest n events of sources with a timestamp in
// [cur.start, cur.end] (epoch milliseconds), newest first.
//
// FilterLogEvents answers oldest-first, so the first pages of a window hold
// its oldest events. Each source is read back from cur.end in windows of
// doubling width, each window whole, until it holds n events: every event in
// a window not yet read is older than every event already held. Across
// sources, a source's events are kept only when newer than every millisecond
// another source left unread. Events sharing the millisecond at the cut are
// all dropped, or all kept when they are the only ones left, so the next read
// never splits them.
//
// next is the Load More token for the read that continues where this one
// stopped, or "" when nothing in [cur.start, cur.end] is left unread. It always
// moves: past the cut, past what was read, to a narrower first window when a
// window did not fit the call budget, or further into a millisecond that did
// not fit it.
func newestLogEvents(ctx context.Context, sources []logSource, cur logCursor, n int) (events []sourcedLogEvent, next string, err error) {
	start := cur.start
	eventTime := func(e sourcedLogEvent) int64 { return aws.ToInt64(e.Timestamp) }
	read := make([][]sourcedLogEvent, len(sources))
	frontiers := make([]logCursor, len(sources))
	unread := logCursor{end: start - 1}
	for i, src := range sources {
		from := cur
		if cur.source != i {
			from.resume = ""
		}
		read[i], frontiers[i], err = readNewest(ctx, src, start, from, n)
		if err != nil {
			return nil, "", err
		}
		frontiers[i].source = i
		if f := frontiers[i]; f.end > unread.end || f.end == unread.end && tighter(f, unread) {
			unread = f
		}
	}
	for i := range sources {
		others := int64(math.MinInt64)
		for j, f := range frontiers {
			if j != i {
				others = max(others, f.end)
			}
		}
		for _, e := range read[i] {
			if eventTime(e) > others {
				events = append(events, e)
			}
		}
	}
	slices.SortStableFunc(events, func(a, b sourcedLogEvent) int { return cmp.Compare(eventTime(b), eventTime(a)) })
	if len(events) > n {
		cut := n
		if boundary := eventTime(events[n-1]); eventTime(events[n]) == boundary {
			cut = slices.IndexFunc(events, func(e sourcedLogEvent) bool { return eventTime(e) == boundary })
			if cut == 0 {
				cut = slices.IndexFunc(events, func(e sourcedLogEvent) bool { return eventTime(e) < boundary })
				if cut < 0 {
					cut = len(events)
				}
			}
		}
		events = events[:cut]
		// Kept at the cut are the events of a millisecond a source is still
		// inside: the next read continues it rather than stepping past it.
		if oldest := eventTime(events[cut-1]); unread.resume == "" || oldest != unread.end {
			unread = logCursor{end: oldest - 1}
		}
	}
	if unread.end >= start {
		unread.start = start
		next = unread.String()
	}
	return events, next, nil
}

// tighter reports whether frontier f, ending at the same millisecond as g,
// is the one to continue from: a millisecond still being read before a
// narrowed window, a narrowed window before a default one.
func tighter(f, g logCursor) bool {
	if f.resume != "" || g.resume != "" {
		return f.resume != "" && g.resume == ""
	}
	return f.span != 0 && (g.span == 0 || f.span < g.span)
}

// readNewest reads one source back from cur until it holds n events, reaches
// start, or runs out of calls. frontier is where its unread range ends.
// A window that does not fit the budget is left unread, and the frontier
// names it with a quarter of its width, so the next read tries a window that
// fits. A single millisecond that alone does not fit keeps what was read of
// it, and the frontier carries the NextToken to continue it with.
func readNewest(ctx context.Context, src logSource, start int64, cur logCursor, n int) (events []sourcedLogEvent, frontier logCursor, err error) {
	calls := 0
	span := cur.span
	if span == 0 {
		span = newestLogFirstSpan.Milliseconds()
	}
	hi, resume := cur.end, cur.resume
	if resume != "" {
		span = 1
	}
	for hi >= start && len(events) < n {
		lo := max(start, hi-span+1)
		window, cursor, err := readLogWindow(ctx, src, lo, hi, resume, &calls)
		if err != nil {
			return nil, logCursor{}, err
		}
		resume = ""
		if cursor != "" && hi > lo {
			return events, logCursor{end: hi, span: max(1, (hi-lo+1)/4)}, nil
		}
		for _, e := range window {
			events = append(events, sourcedLogEvent{FilteredLogEvent: e, region: src.region, group: src.group})
		}
		if cursor != "" {
			return events, logCursor{end: hi, span: 1, resume: cursor}, nil
		}
		hi = lo - 1
		span *= 2
	}
	return events, logCursor{end: hi}, nil
}

// readLogWindow reads the events of src in [lo, hi], from the FilterLogEvents
// NextToken resume when set. cursor is the NextToken to continue with when the
// call budget ran out first, and "" when the window was read to its end;
// events holds what was read either way.
func readLogWindow(ctx context.Context, src logSource, lo, hi int64, resume string, calls *int) (events []cwlogstypes.FilteredLogEvent, cursor string, err error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(src.group),
		StartTime:    aws.Int64(lo),
		EndTime:      aws.Int64(hi),
	}
	if src.pattern != "" {
		input.FilterPattern = aws.String(src.pattern)
	}
	if src.streamPrefix != "" {
		input.LogStreamNamePrefix = aws.String(src.streamPrefix)
	}
	if len(src.streams) > 0 {
		input.LogStreamNames = src.streams
	}
	if resume != "" {
		input.NextToken = aws.String(resume)
	}
	for *calls < maxLogScanCalls {
		out, err := src.api.FilterLogEvents(ctx, input)
		*calls++
		if err != nil {
			return nil, "", err
		}
		events = append(events, out.Events...)
		if aws.ToString(out.NextToken) == "" {
			return events, "", nil
		}
		input.NextToken = out.NextToken
	}
	return events, aws.ToString(input.NextToken), nil
}

// logReadPagination describes a page of rows built from a read that stopped
// at next: truncated, with Load More resuming there, while older events remain.
func logReadPagination(rows int, next string) *resource.PaginationMeta {
	totalHint := rows
	if next != "" {
		totalHint = -1
	}
	return &resource.PaginationMeta{IsTruncated: next != "", NextToken: next, TotalHint: totalHint, PageSize: rows}
}
