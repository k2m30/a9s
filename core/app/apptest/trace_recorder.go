// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package apptest

import (
	"bytes"
	"encoding/json"

	"github.com/k2m30/a9s/v3/core/trace"
)

// TraceRecorder is an io.Writer trace.Enable sink that parses each emitted
// JSON line back into a trace.Event and retains it in arrival order, for
// MonotonicDetailFold/MonotonicCacheWrites to assert against. No locking: a
// Scheduler never spawns a real goroutine (ExecuteTaskAt runs synchronously
// on the caller's own goroutine — see this package's doc comment), so trace
// emission during a driven run is single-threaded despite trace.Emit's own
// internal mutex existing for the general (concurrent) case.
//
// Scope a TraceRecorder to exactly ONE Controller's lifetime. OperationID is
// per-session (session.DetailOpGen starts at 1 for every fresh Controller),
// so a recorder fed events from more than one Controller — e.g. wrapping an
// entire Explorer.Explore() call, which constructs a fresh Controller per
// search-tree node — will compare OperationIDs from unrelated namespaces as
// if they were one history, reporting a violation that is really just two
// independent sessions' op numbering colliding. Driving a single Scheduler
// by hand (one Controller, one recorder) is the direct use of this type;
// Explorer.Trace is the equivalent for the auto-search case — it manages a
// fresh recorder per replay for exactly this reason.
type TraceRecorder struct {
	events []trace.Event
}

// Write implements io.Writer.
func (r *TraceRecorder) Write(p []byte) (int, error) {
	for line := range bytes.SplitSeq(bytes.TrimRight(p, "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var ev trace.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return 0, err
		}
		r.events = append(r.events, ev)
	}
	return len(p), nil
}

// Events returns every event recorded so far, in emission order. The
// returned slice is a defensive copy.
func (r *TraceRecorder) Events() []trace.Event {
	out := make([]trace.Event, len(r.events))
	copy(out, r.events)
	return out
}

// StartTraceRecorder installs a fresh TraceRecorder as core/trace's
// process-wide sink and returns it alongside a restore func the caller must
// defer. core/trace's facility is process-global (see that package's doc
// comment), so a test using this must not run with t.Parallel() alongside
// another test that also enables tracing; restore always disables the
// facility rather than attempting to reinstall a prior sink, matching
// core/trace's own "off by default" contract.
func StartTraceRecorder() (rec *TraceRecorder, restore func()) {
	rec = &TraceRecorder{}
	trace.Enable(rec)
	return rec, trace.Disable
}
