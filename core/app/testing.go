// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/resource"
)

// ApplyResourcesLoaded is a test-support seam that seeds the controller's
// resource cache directly, bypassing the task-result lane. It mirrors the
// task-result handler that runs when resources are fetched via the normal
// task lane. Use this only in tests — the same pattern as DrainSync.
//
// typeName is the canonical short name (e.g. "ec2", "s3").
// appendPage=true accumulates onto the existing cache page; false replaces it.
//
// Mirrors handleResourcesLoadedEvent: the same C6 scope gate (the disk-cache
// save only fires when the top-of-stack screen is the canonical top-level
// ScreenResourceList, not ScreenChildList and not a filtered/related-nav
// list) and the same menu sync-back, so what this seam leaves on the menu and
// on disk is what the real task-result lane leaves.
func (c *Controller) ApplyResourcesLoaded(typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
	// The queued per-type save runs after the lock is released, exactly as
	// Handle runs it (C4) — deferred first so it fires last. This seam then
	// BLOCKS until it has landed: a test that seeds through here reads the
	// file it just wrote on the next line, and the real lane's asynchrony
	// would only make every such test poll for something this seam can wait
	// out. Draining the queue on this goroutine is not that barrier — the
	// writer may already have taken the batch, leaving the drain nothing to
	// do and the file not yet written. Same writes, same order, same
	// function; only the return is later.
	defer c.WaitForCacheWrites()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Resolve canonical short name (handles aliases like "rds" → "dbi"),
	// matching handleResourcesLoadedEvent — the real task-result lane never
	// saves under the raw incoming typeName.
	canon := resource.CanonicalShortName(typeName)
	ls := c.topListState()
	topLevelCanonical := isTopLevelCanonicalList(c.topScreenID(), ls)
	c.applyResourcesLoaded(ls, canon, resources, pagination, appendPage, appendPage, topLevelCanonical, nil, 0)
	if topLevelCanonical && len(c.stack) > 0 {
		// The same second step the real lane takes (handleResourcesLoadedEvent):
		// the menu observation and then the save that records it. Calling only
		// the save would leave this seam persisting an issue count the menu
		// never reconciled — the two answers the round-2 ruling removes.
		c.syncExactTotalToMenu(&c.stack[len(c.stack)-1], canon)
	}
}

// WaitForCacheWrites blocks until every per-type save staged so far has been
// written. Test support, like ApplyResourcesLoaded above: the real lanes hand
// their saves to the cache writer and return, so a test that reads the file a
// call it just made produces has to wait for it — there is nothing else to
// synchronise on, and polling the file cannot tell a stale copy from the new
// one.
func (c *Controller) WaitForCacheWrites() {
	c.wakeCacheWriter()
	c.cacheWriteWG.Wait()
}
