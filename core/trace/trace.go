// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package trace is an opt-in, process-wide structured event stream for the
// detail-operation lifecycle: a fresh open/refresh beginning
// (core/runtime.Core.BeginDetailOperation), the AWS calls it drives
// (core/aws's coalescing decorators — executed vs served-from-memo), the
// on-demand enrich engine's session-scoped cache reads/writes
// (core/aws/detail_enrich_engine.go), and the fold that accepts or rejects
// each async result back in core/app (core/app/handle.go). Ten review rounds
// on this lifecycle found ~50 "action B landed while action A was still in
// flight" defects that thousands of green example-based tests never
// surfaced; this package exists so that sequence can be observed directly
// instead of re-derived from reading code under review.
//
// Every one of those emit sites also runs on the ordinary, undiagnosed path
// (a plain detail open with tracing off), so cost there must be as close to
// zero as an atomic bool load: Enabled() never touches the sink's mutex, and
// every call site with a nontrivial Event field to compute (a rejection
// reason string, a cache-write's accept/refuse verdict) gates that
// computation behind Enabled() too, not just the eventual Emit call.
//
// Deliberately independent of core/aws's CallLedger (call_ledger.go): the
// ledger is a structured, in-process, per-client-construction record a test
// asserts against directly (Records/Duplicates); this package is a
// human/tool-consumable JSON-lines stream, enabled process-wide by the
// cmd/a9s --trace flag or directly by a test via Enable. Both instrument the
// same call sites for the same reason (the ten-round review's defect shape)
// but serve different consumers, so neither depends on the other.
//
// core/aws, core/runtime and core/app all need to emit from their own call
// sites, so this package must sit below all three with no import-cycle risk
// — it has zero a9s-internal imports (stdlib only), the same leaf position
// core/domain occupies.
package trace

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Kind identifies which lifecycle moment an Event describes.
type Kind string

const (
	KindDetailOpBegin Kind = "detail_op_begin"
	KindAWSCall       Kind = "aws_call"
	KindCache         Kind = "cache"
	KindFold          Kind = "fold"
)

// Event is the one structured record every emit site produces. Fields are a
// superset across every Kind; each Kind only populates the fields relevant
// to it, so the rest stay zero and are omitted from the marshaled JSON line.
type Event struct {
	Time time.Time `json:"time"`
	Kind Kind      `json:"kind"`

	// OperationID is the core/runtime.DetailOperation.ID this event belongs
	// to, populated on every Kind — it is the one thread that ties a
	// detail_op_begin event to every aws_call/cache/fold event the same
	// operation subsequently produces. 0 means no active detail operation
	// (e.g. a non-operation AWS call, or a fold check with a zero
	// GenStamp — see messages.GenStamped.AcceptZeroGen's doc comment for why
	// that is a legitimate, non-operation value, not an error).
	OperationID uint64 `json:"operation_id,omitempty"`

	// KindDetailOpBegin
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Refresh      bool   `json:"refresh,omitempty"`

	// KindAWSCall
	API     string `json:"api,omitempty"`
	Args    string `json:"args,omitempty"`
	Outcome string `json:"outcome,omitempty"` // "executed" | "served"

	// KindCache
	Cache        string `json:"cache,omitempty"` // "policy" | "detail" | "unknown"
	Key          string `json:"key,omitempty"`
	CacheOutcome string `json:"cache_outcome,omitempty"` // "hit" | "miss" | "write"

	// KindFold
	EventType string `json:"event_type,omitempty"` // e.g. "EnrichDetailResult"
	Accepted  bool   `json:"accepted,omitempty"`

	// Reason is set on a KindFold rejection (why the result was stale) or a
	// KindCache write (whether opAwareDocStore.setIfNewer's freshness guard
	// accepted or refused it) — the two moments a human reading a trace most
	// needs the "why", not just the "what".
	Reason string `json:"reason,omitempty"`
}

var (
	mu   sync.Mutex
	sink io.Writer
	on   atomic.Bool
)

// Enable installs w as the trace sink; a nil w disables the facility
// (equivalent to Disable). Safe for concurrent use with Emit/Enabled; like
// core/logging.Setup, not safe to call concurrently with itself.
func Enable(w io.Writer) {
	mu.Lock()
	sink = w
	mu.Unlock()
	on.Store(w != nil)
}

// EnableFile opens path (creating if necessary, mode 0600) for append and
// installs it as the trace sink — cmd/a9s's --trace flag entry point.
// Mirrors core/logging.Setup: call once at startup, before the TUI takes the
// terminal or the web server starts accepting connections, so trace output
// is a plain file write that never interleaves with the rendered frame or
// an HTTP response body. On success, close releases the opened file
// (callers should defer it, matching core/logging.Setup's contract).
func EnableFile(path string) (close func() error, err error) {
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if openErr != nil {
		return nil, openErr
	}
	Enable(f)
	return f.Close, nil
}

// Disable turns the facility off; Emit becomes a no-op again.
func Disable() {
	Enable(nil)
}

// Enabled reports whether the facility is currently on. A single
// sync/atomic.Bool load, no lock — a call site with a nontrivial Event field
// to build (e.g. a rejection reason string) should gate that computation,
// and the surrounding Emit call, behind this check so the cost of building
// the value is also skipped when tracing is off, not just the eventual
// write.
func Enabled() bool {
	return on.Load()
}

// Emit writes ev as one JSON line to the installed sink. No-op — one atomic
// bool load, nothing else — when disabled. Safe for concurrent use: the
// related-checker fan-out this package exists to trace calls it from
// multiple goroutines under one detail operation, and the underlying sink
// (e.g. a test's bytes.Buffer) is not assumed to be concurrency-safe on its
// own, so the write itself is serialized here.
func Emit(ev Event) {
	if !on.Load() {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	b = append(b, '\n')
	mu.Lock()
	defer mu.Unlock()
	if sink == nil {
		return
	}
	_, _ = sink.Write(b)
}
