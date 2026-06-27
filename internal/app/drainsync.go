package app

import "github.com/k2m30/a9s/v3/internal/runtime"

// DrainSync runs the pending task slice to completion synchronously.
// It loops while pending is non-empty, popping one TaskRequest per iteration.
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
	for len(pending) > 0 {
		// Pop one task from the front.
		_, pending = pending[0], pending[1:]
		// TODO PR-B0: execute the TaskRequest via the extracted TaskExecutor,
		// then pending = append(pending, c.Handle(resultEvent)...).
		// For now the task is dropped (drains to empty) without execution.
	}
}
