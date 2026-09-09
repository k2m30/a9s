// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_flush_lock_scope_test.go — no reader waits on a lock the flush
// holds across the encode.
//
// The ratio pin this replaces measured the scheduler: encoding a six-thousand
// row type file costs a concurrent reader's worst snapshot 5-8x with no lock
// held at all, from one garbage-collection assist charged to whichever
// goroutine allocates next. A latency bound cannot separate that from a lock,
// so this states the property directly and has no clock in it.
//
// Failure mode: if a lock IS held across the encode, the two calls below never
// return and the test binary's own timeout prints the goroutine dump naming
// the mutex. That is the diagnostic; a watchdog here would only be a clock
// with a nicer message.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestFlushCacheWrites_HoldsNoLockAcrossTheEncode parks the flush inside the
// encode and drives the two things the operator's key press needs: the
// controller's own snapshot, and the session pair lock every cache decision
// resolves under.
func TestFlushCacheWrites_HoldsNoLockAcrossTheEncode(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	entered := make(chan struct{})
	release := make(chan struct{})
	restore := cache.SetEncodeHookForTest(func() {
		close(entered)
		<-release
	})
	defer restore()

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(6000),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	flushed := make(chan struct{})
	go func() {
		c.WaitForCacheWrites()
		close(flushed)
	}()
	<-entered

	// The frame the renderer builds for a key press.
	if c.Snapshot().Body.List == nil {
		t.Error("the snapshot taken while the flush was encoding carries no list body")
	}
	// The lock every profile/region-scoped cache decision resolves under. A
	// flush that still held it here would be the 60ms hold the row is about.
	if profile, _ := core.Session().CurrentPair(); profile == "" {
		t.Error("the session pair read while the flush was encoding is empty")
	}

	close(release)
	<-flushed
}
