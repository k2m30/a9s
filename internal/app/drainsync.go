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
// and have no meaning in a headless sync context.
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
	iterations := 0
	for len(pending) > 0 {
		if iterations >= maxDrainIterations {
			break
		}
		iterations++

		req := pending[0]
		pending = pending[1:]

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
	}
}
