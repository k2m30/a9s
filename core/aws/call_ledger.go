// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// call_ledger.go makes coalesce.go's "at most one call per (operation, api,
// arguments)" contract mechanically checkable instead of documented. Off by
// default and free when off: every coalescing decorator holds a *CallLedger
// field that stays nil unless a caller explicitly constructs one via the
// WithLedger constructor variants below — recording a call then costs one
// nil-pointer receiver check on the hot path, no lock, no allocation, the
// same "pay only if you opted in" shape completedResultMemo already uses for
// its own per-operation bookkeeping.
package aws

import (
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
)

// CallOutcome distinguishes the two ways a coalescing decorator answers a
// call without necessarily making a new AWS request.
type CallOutcome int

const (
	// CallExecuted means the underlying AWS API call actually fired — once,
	// regardless of how many concurrent identical calls singleflight.Group
	// joined to it; only the one goroutine whose closure ran records this.
	CallExecuted CallOutcome = iota
	// CallServed means the call was answered with no new AWS request at
	// all, either from completedResultMemo (a call arriving after an
	// earlier flight for this operation already completed) or by joining
	// another goroutine's still-in-flight singleflight.Group call for the
	// same key (two concurrent askers, one AWS request). Both are recorded
	// so the ledger can see every asker, not just the one that executed —
	// without a record for the joiner, "one call, two askers" (correct
	// dedup) and "one call, one asker" (the other asker's request never
	// happened at all — a real bug) would look identical.
	CallServed
)

// CallRecord is one call site recorded by a CallLedger. Args is the exact
// per-call argument value (StateMachineArn/TopicArn/Bucket/FunctionName)
// each coalescing decorator already computes as coalesceKey's second
// parameter — reused verbatim here, not re-derived.
type CallRecord struct {
	Operation domain.Gen
	API       string
	Args      string
	Outcome   CallOutcome
}

// CallLedger records every call a coalescing decorator makes when
// constructed with one (NewCoalescingSFNWithLedger and its three siblings).
// A nil *CallLedger is the default: every method below tolerates a nil
// receiver as a silent no-op, so an undecorated call site pays nothing.
type CallLedger struct {
	mu      sync.Mutex
	records []CallRecord
}

// NewCallLedger returns an empty, ready-to-use CallLedger.
func NewCallLedger() *CallLedger {
	return &CallLedger{}
}

// record appends one CallRecord. Safe on a nil receiver (no-op).
func (l *CallLedger) record(operation domain.Gen, api, args string, outcome CallOutcome) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.records = append(l.records, CallRecord{Operation: operation, API: api, Args: args, Outcome: outcome})
	l.mu.Unlock()
}

// Records returns every call recorded so far, in dispatch order. Safe on a
// nil receiver (returns nil).
func (l *CallLedger) Records() []CallRecord {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]CallRecord, len(l.records))
	copy(out, l.records)
	return out
}

// Duplicates returns every (Operation, API, Args) key whose underlying AWS
// call EXECUTED more than once, keyed by a CallRecord with Outcome ==
// CallExecuted, mapped to how many times it executed. This is the ledger's
// answer to "at most one call per (operation, api, arguments)": a
// CallServed entry sharing the same key is evidence the memo did its job
// (the point of recording Served separately from Executed at all), not a
// second call, so it is deliberately excluded from this count — otherwise
// every correctly-deduplicated repeat lookup would misreport as a
// duplicate. Operation == 0 (no active DetailOperation) is also excluded:
// that traffic is never coalesced by design, so repeated fetches under it
// are ordinary behavior, not a defect. Safe on a nil receiver (returns nil).
func (l *CallLedger) Duplicates() map[CallRecord]int {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	counts := make(map[CallRecord]int)
	for _, r := range l.records {
		if r.Operation == 0 || r.Outcome != CallExecuted {
			continue
		}
		counts[CallRecord{Operation: r.Operation, API: r.API, Args: r.Args, Outcome: CallExecuted}]++
	}
	dups := make(map[CallRecord]int)
	for k, c := range counts {
		if c > 1 {
			dups[k] = c
		}
	}
	return dups
}
