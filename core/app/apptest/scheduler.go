// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package apptest is the deterministic interleaving harness for core/app's
// detail-operation orchestration (GitHub issue #488, item 1). It exists to
// make "action B lands while action A is still in flight" reachable by a
// test: core/app's Drain* variants (drainsync.go) always execute the front
// of a FIFO queue, so a task dispatched earlier can never be observed
// completing AFTER one dispatched later — precisely the shape of the ~50
// defects the ten detail-enrichment review rounds found. This package adds
// the seam Drain* deliberately does not: a Scheduler that lets a caller
// choose which pending task completes next, and an Explorer that enumerates
// every legal choice up to a bounded depth.
//
// Not a virtual clock: nothing in the detail-operation path (BeginDetailOperation,
// beginDetailWorkloadLocked, ExecuteTaskAt's KindEnrichDetail/KindRelatedCheck
// cases, the fold logic in core/app/handle.go, session.PendingDetailRefresh,
// core/aws/coalesce.go's singleflight/memo) reads a wall clock to decide
// ordering, a timeout, or acceptance — the only time.Now() reads anywhere
// near this path (core/runtime/executor.go's availability-probe cases, and
// costs' ensureCostsState) are metrics or an unrelated domain, never branched
// on. Ordering here is entirely which TaskRequest a caller executes next, so
// no clock injection exists in this package.
//
// This package lives outside core/app (rather than in a non-test file in
// package app, where DrainSync and testing.go's ApplyResourcesLoaded already
// live) because it is substantially larger, purpose-built test machinery —
// unlike those two small stable seams, it has no reason to ship in the a9s
// binary. No file in cmd/ or internal/ imports core/app/apptest; only
// tests/unit does, so the import graph proves it absent from the production
// binary path rather than merely leaving it uncalled within a shared package.
package apptest

import (
	"context"
	"errors"
	"fmt"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// Scheduler is the pluggable seam that replaces Drain*'s "always execute
// pending[0]" policy with "the caller picks the index." Tasks a user action
// spawns are Submitted as pending — in flight from the scheduler's
// perspective — until Complete steps one of them, at whatever index the
// caller chooses, including an index submitted in an earlier Submit call
// after a later one already completed.
type Scheduler struct {
	ctrl    *app.Controller
	ctx     context.Context
	pending []runtime.TaskRequest
}

// NewScheduler wraps ctrl. ctx is forwarded to every Controller.ExecuteOne
// call Complete makes; a caller bounding total execution time should supply
// a context carrying that deadline.
func NewScheduler(ctx context.Context, ctrl *app.Controller) *Scheduler {
	return &Scheduler{ctrl: ctrl, ctx: ctx}
}

// Submit enqueues tasks a user action produced — the []runtime.TaskRequest
// already returned by Controller.Apply, Controller.Handle, or
// Controller.BeginDetailWorkload — as pending.
func (s *Scheduler) Submit(tasks ...runtime.TaskRequest) {
	s.pending = append(s.pending, tasks...)
}

// Pending returns the current pending queue in submission order — index i
// is the value Complete(i) would step next. The returned slice is a
// defensive copy; mutating it does not affect the scheduler.
func (s *Scheduler) Pending() []runtime.TaskRequest {
	out := make([]runtime.TaskRequest, len(s.pending))
	copy(out, s.pending)
	return out
}

// InFlight reports whether any pending task carries the given TaskKind.
func (s *Scheduler) InFlight(kind runtime.TaskKind) bool {
	for _, t := range s.pending {
		if t.Key.Kind == kind {
			return true
		}
	}
	return false
}

// Quiescent reports whether every submitted task has completed.
func (s *Scheduler) Quiescent() bool {
	return len(s.pending) == 0
}

// Complete steps the pending task at index i to completion: the identical
// execute-then-fold unit Controller.ExecuteOne performs for every Drain*
// variant, except the caller supplies the index instead of the queue always
// picking the front. Any follow-up tasks the step produces are appended to
// the end of pending — a fresh Submit, indistinguishable from one the test
// issued itself, so a follow-up chain interleaves under the same rules as
// everything else.
//
// An adapter-only kind (runtime.ErrAdapterOnlyTask) is not an error from the
// caller's perspective — it is simply not runnable outside a renderer, the
// same distinction Drain* already makes — so it is swallowed here. Any other
// execution error is returned: unlike Drain*'s "keep going no matter what"
// production posture, a driven interleaving test wants to know immediately
// when something broke.
func (s *Scheduler) Complete(i int) error {
	if i < 0 || i >= len(s.pending) {
		return fmt.Errorf("apptest: no pending task at index %d (%d pending)", i, len(s.pending))
	}
	req := s.pending[i]
	s.pending = append(append([]runtime.TaskRequest{}, s.pending[:i]...), s.pending[i+1:]...)

	followUp, _, err := s.ctrl.ExecuteOne(s.ctx, req)
	if err != nil {
		if errors.Is(err, runtime.ErrAdapterOnlyTask) {
			return nil
		}
		return err
	}
	s.pending = append(s.pending, followUp...)
	return nil
}

// DrainRemaining completes every still-pending task in FIFO order — for a
// test that drives one specific reordering deliberately and wants everything
// else to finish normally afterward, mirroring core/app.DrainSync's own
// queue order for whatever is left.
func (s *Scheduler) DrainRemaining() error {
	for !s.Quiescent() {
		if err := s.Complete(0); err != nil {
			return err
		}
	}
	return nil
}
