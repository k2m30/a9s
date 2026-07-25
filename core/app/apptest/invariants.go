// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package apptest

import (
	"fmt"
	"sort"
	"strings"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/trace"
)

// NoOrphanLoadingRelated asserts that a detail screen never shows a related-
// panel row still marked Loading once nothing pending could ever resolve it.
// Coarse-grained by design: one RelatedCheckPayload task resolves every
// registered RelatedDef for a resource in a single dispatch (there is no
// per-target-type task to match exactly against), so this checks for ANY
// pending runtime.KindRelatedCheck task rather than one scoped to the exact
// resource — sufficient because only one screen is ever the top of the
// stack, so only its own related rows can show Loading at all.
func NoOrphanLoadingRelated(vs app.ViewState, pending []runtime.TaskRequest) error {
	if vs.Body.Kind != app.BodyKindDetail || vs.Body.Detail == nil {
		return nil
	}
	var stuck []string
	for _, row := range vs.Body.Detail.Related {
		if row.Loading {
			stuck = append(stuck, row.Name)
		}
	}
	if len(stuck) == 0 {
		return nil
	}
	for _, t := range pending {
		if t.Key.Kind == runtime.KindRelatedCheck {
			return nil
		}
	}
	return fmt.Errorf("apptest: related row(s) %s show Loading with no related-check task in flight", strings.Join(stuck, ", "))
}

// MonotonicDetailFold asserts that, among ACCEPTED trace.KindFold events (one
// per RelatedCheckBatch/RelatedCheckResult/EnrichDetailResult fold decision —
// see core/app/handle.go's traceFoldAcceptance), OperationID never regresses
// in fold order. Core.ActiveDetailOp() (session.DetailOpGen) only advances,
// and messages.IsStale's equality check means an ACCEPTED fold's OperationID
// equals whatever ActiveDetailOp was at that exact moment — so a later-folded
// accepted event carrying an OLDER OperationID than one already accepted is
// evidence the acceptance guard was bypassed on some code path, not merely a
// stale value arriving late (IsStale already rejects that case; this checks
// that it actually held for every fold the run produced, not just the ones a
// hand-written example test happened to exercise).
//
// events must cover exactly ONE Controller's lifetime — either
// StartTraceRecorder wrapped around a single hand-driven Scheduler run, or
// the per-replay events Explorer passes to Check when Explorer.Trace is
// true. OperationID is per-session (starts at 1 for every fresh Controller),
// so events spanning more than one Controller (e.g. one recorder wrapped
// around an entire Explorer.Explore() call, which builds a fresh Controller
// per search-tree node) will report a false violation — two independent
// sessions' unrelated op numbering, not a real regression. See
// TraceRecorder's doc comment.
func MonotonicDetailFold(events []trace.Event) error {
	var lastOp uint64
	var lastType string
	for _, ev := range events {
		if ev.Kind != trace.KindFold || !ev.Accepted {
			continue
		}
		if ev.OperationID < lastOp {
			return fmt.Errorf("apptest: accepted fold %s(op=%d) landed after %s(op=%d) already satisfied — visible detail regressed to an older operation",
				ev.EventType, ev.OperationID, lastType, lastOp)
		}
		lastOp = ev.OperationID
		lastType = ev.EventType
	}
	return nil
}

// MonotonicCacheWrites asserts that, per (Cache, Key) pair, trace.KindCache
// "write" events never regress OperationID. The opAwareDocStore.setIfNewer
// freshness guard (see core/trace's package doc) exists precisely to make a
// stale write impossible; this checks the guard actually held for every key
// the run touched, not just ones covered by a hand-written example test.
//
// events must cover exactly ONE Controller's lifetime — see
// MonotonicDetailFold's doc comment for why (OperationID is per-session and
// mixing sessions produces a false violation).
func MonotonicCacheWrites(events []trace.Event) error {
	last := make(map[string]trace.Event)
	for _, ev := range events {
		if ev.Kind != trace.KindCache || ev.CacheOutcome != "write" {
			continue
		}
		key := ev.Cache + "/" + ev.Key
		if prev, ok := last[key]; ok && ev.OperationID < prev.OperationID {
			return fmt.Errorf("apptest: cache write to %s by operation %d landed after operation %d already wrote it — stale write accepted", key, ev.OperationID, prev.OperationID)
		}
		last[key] = ev
	}
	return nil
}

// NoDuplicateAWSCalls wraps aws.CallLedger.Duplicates() as a descriptive
// failure instead of a bare map the caller would otherwise have to format
// itself — the ledger already IS the "at most one call per (operation, api,
// args)" mechanism (core/aws/call_ledger.go); this only gives its answer the
// same "descriptive failure, not a bool" shape as every other check here.
func NoDuplicateAWSCalls(ledger *awsclient.CallLedger) error {
	dups := ledger.Duplicates()
	if len(dups) == 0 {
		return nil
	}
	lines := make([]string, 0, len(dups))
	for k, n := range dups {
		lines = append(lines, fmt.Sprintf("op=%d %s(%s) executed %d times", k.Operation, k.API, k.Args, n))
	}
	sort.Strings(lines)
	return fmt.Errorf("apptest: duplicate AWS calls: %s", strings.Join(lines, "; "))
}

// LatchCleared asserts that a refresh-armed session.PendingDetailRefresh
// entry (core/app/navigate.go's beginDetailWorkloadLocked "sticky refresh"
// arm — see that method's doc comment) was cleared by the time the run
// reached quiescence. The only clear path is a successful, at-least-as-new
// EnrichDetailResult fold (foldEnrichDetailResultLocked); an entry still
// armed once every spawned task has completed can never clear on its own,
// since nothing else in the system revisits it.
//
// key is runtime.RelatedCacheKey(resourceType, resourceID) — the same key
// beginDetailWorkloadLocked arms under.
func LatchCleared(ctrl *app.Controller, key string) error {
	if opID, armed := ctrl.PendingDetailRefreshGet(key); armed {
		return fmt.Errorf("apptest: PendingDetailRefresh(%s) still armed by operation %d at quiescence", key, opID)
	}
	return nil
}
