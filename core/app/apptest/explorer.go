// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package apptest

import (
	"context"
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/trace"
)

// Action is one user-driven step the Explorer applies, in a fixed order, to
// a fresh Controller. Name is the replay label ("open(ec2/i-1)", "refresh")
// — the Explorer never invents one, since only the caller knows what a step
// means. Run performs the step (typically Controller.Apply or
// Controller.BeginDetailWorkload) and returns the tasks it spawned; the
// Explorer submits those to its Scheduler exactly as Apply/Handle callers
// already do.
type Action struct {
	Name string
	Run  func(ctx context.Context, ctrl *app.Controller) []runtime.TaskRequest
}

// TaskLabelFunc names one pending task for a replay string. See
// DefaultTaskLabel for the label every Explorer uses unless overridden.
type TaskLabelFunc func(runtime.TaskRequest) string

// DefaultTaskLabel renders "enrich@opN" / "related@opN" for the two
// detail-scope kinds (using the DetailOperation.ID runtime.TaskOpID already
// exposes) and the bare TaskKind string for everything else.
func DefaultTaskLabel(req runtime.TaskRequest) string {
	short := string(req.Key.Kind)
	switch req.Key.Kind {
	case runtime.KindEnrichDetail:
		short = "enrich"
	case runtime.KindRelatedCheck:
		short = "related"
	}
	if opID := runtime.TaskOpID(req.Payload); opID != 0 {
		return fmt.Sprintf("%s@op%d", short, opID)
	}
	return short
}

// Check runs after every applied Action and every completed task, against
// the controller's current Snapshot(), the scheduler's current pending
// queue, and (only when Explorer.Trace is true) the trace.Event stream
// recorded so far for THIS replay's own Controller — see Explorer.Trace.
// events is nil when Explorer.Trace is false. Return a non-nil error to
// fail the interleaving at that exact step.
type Check func(vs app.ViewState, pending []runtime.TaskRequest, events []trace.Event) error

// Explorer enumerates every legal interleaving of a fixed Action sequence
// against the completion order of the tasks those actions spawn, running
// each interleaving against its own fresh Controller (via NewController) to
// quiescence.
//
// "Legal" means: an Action never runs before the one before it (Actions are
// a script, not a choice), and a task never completes before the Action that
// spawned it has run — everything else is free to interleave, including
// completing a task an EARLIER action spawned strictly after a LATER action
// (and any of ITS tasks) has already completed.
type Explorer struct {
	// NewController builds a fresh Controller in its starting state. Called
	// once per candidate interleaving (an Explore call fully replays a
	// candidate's move sequence from a new Controller rather than trying to
	// backtrack a shared one, since Controller state has no undo).
	NewController func() *app.Controller
	Actions       []Action
	// TaskLabel names a task for the replay string; nil uses DefaultTaskLabel.
	TaskLabel TaskLabelFunc
	Check     Check
	// Trace, when true, has the Explorer install its own TraceRecorder
	// around EVERY replay it runs — one per search-tree node, each against
	// its own fresh Controller from NewController — and pass that replay's
	// OWN events (never another replay's) to Check.
	//
	// core/trace is process-wide (see that package's doc comment) and a
	// fresh Controller's session starts DetailOpGen back at 1, so a caller
	// who instead wraps a whole Explore() call in one manually-installed
	// StartTraceRecorder would accumulate events from many independent
	// Controllers into one stream — MonotonicDetailFold/MonotonicCacheWrites
	// would then compare OperationIDs from entirely unrelated op-ID
	// namespaces as if they were one history, reporting a violation that
	// is really just two different sessions' unrelated op numbering
	// colliding. Setting Trace true instead of managing a recorder
	// yourself avoids that false positive by construction: the Explorer
	// resets the recorder at the start of every replay, exactly matching
	// each replay's own single-Controller scope.
	Trace bool
}

// Failure is returned by Explore for the first interleaving whose Check (or
// whose task/action execution) failed. Replay is a human-readable,
// directly-reproducible sequence, e.g.:
//
//	open(ec2/i-1) → refresh → complete(enrich@op1) → complete(related@op2)
type Failure struct {
	Replay string
	Err    error
}

func (f *Failure) Error() string {
	return fmt.Sprintf("%s: %v", f.Replay, f.Err)
}

type moveKind int

const (
	moveApplyAction moveKind = iota
	moveComplete
)

// move is one step in a candidate interleaving. idx is the pending-queue
// index moveComplete resolves; meaningless for moveApplyAction.
type move struct {
	kind moveKind
	idx  int
}

// Explore runs every legal interleaving depth-first, stopping at the first
// one whose Check reports a violation (or whose task/action execution
// itself errors). Nil means every interleaving reached quiescence clean.
//
// Cost: replays the ENTIRE candidate sequence from a fresh Controller at
// every node in the search tree, rather than incrementally extending a
// shared one. Task counts are interleaving-dependent — beginDetailWorkloadLocked's
// cache-replay suppression can make the SAME action spawn fewer tasks once
// an earlier interleaving already completed its related-check — so a task
// count discovered along one path cannot be assumed valid on another; only a
// full replay from the actual starting state is guaranteed correct.
// ponytail: O(nodes × depth) rather than incremental/backtracking state;
// fine at the "3-4 actions" scale this Explorer targets — revisit with
// memoized per-prefix Controller snapshots only if real usage needs
// materially greater depth.
func (e *Explorer) Explore(ctx context.Context) *Failure {
	return e.explore(ctx, nil)
}

func (e *Explorer) explore(ctx context.Context, prefix []move) *Failure {
	sched, actionsApplied, f := e.runSequence(ctx, prefix)
	if f != nil {
		return f
	}
	pending := sched.Pending()
	moreActions := actionsApplied < len(e.Actions)
	if !moreActions && len(pending) == 0 {
		return nil
	}
	if moreActions {
		next := append(append([]move{}, prefix...), move{kind: moveApplyAction})
		if f := e.explore(ctx, next); f != nil {
			return f
		}
	}
	for i := range pending {
		next := append(append([]move{}, prefix...), move{kind: moveComplete, idx: i})
		if f := e.explore(ctx, next); f != nil {
			return f
		}
	}
	return nil
}

// runSequence replays moves from a fresh Controller, calling e.Check after
// every step. Returns the reached (scheduler, actionsApplied) state on a
// clean replay, or a *Failure at the first step whose Check — or whose
// action/task execution — errors.
func (e *Explorer) runSequence(ctx context.Context, moves []move) (*Scheduler, int, *Failure) {
	ctrl := e.NewController()
	sched := NewScheduler(ctx, ctrl)
	actionsApplied := 0
	var replay []string

	var rec *TraceRecorder
	if e.Trace {
		rec = &TraceRecorder{}
		trace.Enable(rec)
		defer trace.Disable()
	}

	check := func(step string) *Failure {
		replay = append(replay, step)
		if e.Check == nil {
			return nil
		}
		var events []trace.Event
		if rec != nil {
			events = rec.Events()
		}
		if err := e.Check(ctrl.Snapshot(), sched.Pending(), events); err != nil {
			return &Failure{Replay: strings.Join(replay, " → "), Err: err}
		}
		return nil
	}

	label := e.TaskLabel
	if label == nil {
		label = DefaultTaskLabel
	}

	for _, m := range moves {
		switch m.kind {
		case moveApplyAction:
			a := e.Actions[actionsApplied]
			tasks := a.Run(ctx, ctrl)
			sched.Submit(tasks...)
			actionsApplied++
			if f := check(a.Name); f != nil {
				return nil, 0, f
			}

		case moveComplete:
			pending := sched.Pending()
			if m.idx < 0 || m.idx >= len(pending) {
				replay = append(replay, "complete(<out-of-range>)")
				return nil, 0, &Failure{Replay: strings.Join(replay, " → "), Err: fmt.Errorf("apptest: move index %d out of range (%d pending)", m.idx, len(pending))}
			}
			step := "complete(" + label(pending[m.idx]) + ")"
			if err := sched.Complete(m.idx); err != nil {
				replay = append(replay, step)
				return nil, 0, &Failure{Replay: strings.Join(replay, " → "), Err: err}
			}
			if f := check(step); f != nil {
				return nil, 0, f
			}
		}
	}
	return sched, actionsApplied, nil
}
