package app

import (
	"context"
	"errors"

	"github.com/k2m30/a9s/v3/internal/runtime"
)

// maxDrainIterations caps the total number of task executions in a single
// DrainSync call. A chain of N resources dispatching M follow-up tasks each
// would grow exponentially without this guard. 10 000 iterations is large
// enough to drain any realistic fixture graph in tests while preventing
// accidental infinite loops if a handler emits the same task kind repeatedly.
const maxDrainIterations = 10_000

// DrainSync runs the pending task slice to completion synchronously.
// It loops while pending is non-empty, executing each TaskRequest via
// Core.ExecuteTask and feeding the result event through Handle to collect
// any follow-up tasks. Adapter-only kinds (those for which ExecuteTask
// returns ErrAdapterOnlyTask) are skipped — they are renderer concerns
// and have no meaning in a headless sync context — with one exception:
// TaskKindEmitNavigate is routed through Controller.ApplyEmitNavigate so the
// one-shot -c/ActionCommand navigation still lands on the headless stack.
//
// This is the testing keystone: tests call Apply (or Handle) to get an initial
// pending slice, then pass it to DrainSync to run tasks inline without a
// terminal, goroutines, or sleeps. In demo/fake mode the underlying fetchers
// are synchronous, so DrainSync is fully deterministic.
//
// DrainSync lives in a non-test file so it can be imported by the web
// integration test package without triggering Go's test-package import
// restrictions.
func DrainSync(c *Controller, pending []runtime.TaskRequest) {
	DrainSyncContext(context.Background(), c, pending)
}

// DrainSyncContext is the context-aware variant of DrainSync. ctx is
// forwarded to every Core.ExecuteTask call; callers should supply a context
// with an appropriate deadline when execution time must be bounded.
func DrainSyncContext(ctx context.Context, c *Controller, pending []runtime.TaskRequest) {
	DrainSyncContextProgress(ctx, c, pending, nil)
}

// DrainSyncProgress is like DrainSync but invokes onEvent after each handled
// event, letting a caller push incremental UI updates (e.g. SSE notifications)
// as results arrive instead of only once at the end. This is what keeps a live
// (non-demo) web session's menu filling in progressively rather than staying
// blank for the entire — slow — availability drain. onEvent may be nil.
func DrainSyncProgress(c *Controller, pending []runtime.TaskRequest, onEvent func()) {
	DrainSyncContextProgress(context.Background(), c, pending, onEvent)
}

// DrainSyncContextProgress is the context-aware progress variant. The loop is
// identical to the plain drain but calls onEvent (when non-nil) after each
// event is handled, so the controller state is observable as it fills in.
func DrainSyncContextProgress(ctx context.Context, c *Controller, pending []runtime.TaskRequest, onEvent func()) {
	iterations := 0
	for len(pending) > 0 {
		if iterations >= maxDrainIterations {
			break
		}
		iterations++

		req := pending[0]
		pending = pending[1:]

		if req.Key.Kind == runtime.TaskKindEmitNavigate {
			// Adapter-only from Core.ExecuteTask's perspective — the TUI
			// intercepts this kind before ExecuteTask and translates it into
			// a view-stack push (runtime_adapter.go's emitNavigateCmd); the
			// headless/web lane does the same via Controller.ApplyEmitNavigate
			// so the one-shot -c navigation is not silently dropped.
			if p, ok := req.Payload.(runtime.EmitNavigatePayload); ok {
				followUp := c.ApplyEmitNavigate(p)
				pending = append(pending, followUp...)
				if onEvent != nil {
					onEvent()
				}
			}
			continue
		}

		ev, err := c.core.ExecuteTask(ctx, req)
		if err != nil {
			if errors.Is(err, runtime.ErrAdapterOnlyTask) {
				// Renderer-only kind — irrelevant in a headless sync context.
				continue
			}
			// Execution error: no event to dispatch, no follow-up tasks.
			continue
		}
		if ev == nil {
			// Task completed with no result to dispatch (e.g. save-cache no-op).
			continue
		}

		_, followUp := c.Handle(ev)
		pending = append(pending, followUp...)
		if onEvent != nil {
			onEvent()
		}
	}
}

// DrainSyncPartition behaves like DrainSyncContextProgress but partitions the
// pending queue by isBackground: tasks classified background (isBackground
// returns true) are collected — never executed — and returned to the caller
// instead of being run inline, INCLUDING follow-up tasks emitted by tasks
// that WERE executed. Tasks classified blocking (isBackground returns false)
// run exactly as DrainSyncContextProgress would: executed via
// Core.ExecuteTask, results fed through Controller.Handle, with any
// follow-ups re-enqueued (subject to the same background/blocking split).
//
// This lets a caller (e.g. a web request handler) drain only the tasks whose
// result the response needs (blocking) and hand background tasks (related-
// check fan-out, detail enrichment, save-cache) to a goroutine that completes
// after the response has already been written.
//
// onEvent fires once per executed (blocking) task result — mirroring
// DrainSyncProgress semantics — and is never invoked for tasks that were
// deferred as background without executing. onEvent may be nil.
func DrainSyncPartition(
	ctx context.Context,
	c *Controller,
	pending []runtime.TaskRequest,
	isBackground func(runtime.TaskKind) bool,
	onEvent func(),
) []runtime.TaskRequest {
	var deferred []runtime.TaskRequest

	iterations := 0
	for len(pending) > 0 {
		if iterations >= maxDrainIterations {
			break
		}
		iterations++

		req := pending[0]
		pending = pending[1:]

		if isBackground != nil && isBackground(req.Key.Kind) {
			deferred = append(deferred, req)
			continue
		}

		if req.Key.Kind == runtime.TaskKindEmitNavigate {
			// See DrainSyncContextProgress — adapter-only from
			// Core.ExecuteTask's perspective; route through
			// Controller.ApplyEmitNavigate instead of dropping it.
			if p, ok := req.Payload.(runtime.EmitNavigatePayload); ok {
				followUp := c.ApplyEmitNavigate(p)
				pending = append(pending, followUp...)
				if onEvent != nil {
					onEvent()
				}
			}
			continue
		}

		ev, err := c.core.ExecuteTask(ctx, req)
		if err != nil {
			if errors.Is(err, runtime.ErrAdapterOnlyTask) {
				// Renderer-only kind — irrelevant in a headless sync context.
				continue
			}
			// Execution error: no event to dispatch, no follow-up tasks.
			continue
		}
		if ev == nil {
			// Task completed with no result to dispatch (e.g. save-cache no-op).
			continue
		}

		_, followUp := c.Handle(ev)
		pending = append(pending, followUp...)
		if onEvent != nil {
			onEvent()
		}
	}

	return deferred
}
